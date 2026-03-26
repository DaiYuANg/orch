use std::collections::{HashMap, HashSet};
use warden_dsl_ast::{ExprAst, PathAst, StmtAst};
use warden_dsl_hir::HirDocument;
mod types;

pub use types::*;

pub fn bind(hir: &HirDocument) -> Result<BoundDocument, BindError> {
  let mut bindings = HashMap::new();
  let mut names = HashSet::new();
  let mut volume_names = HashSet::new();
  let mut config_names = HashSet::new();
  let mut secret_names = HashSet::new();
  let endpoint_names = hir
    .services
    .iter()
    .map(|service| (service.name.clone(), endpoint_name(&service.body)))
    .collect::<HashMap<_, _>>();
  for service in &hir.services {
    if bindings
      .insert(service.binding.to_string(), service.name.clone())
      .is_some()
    {
      return Err(BindError::DuplicateBinding(service.binding.to_string()));
    }
    names.insert(service.name.clone());
  }
  for volume in &hir.volumes {
    volume_names.insert(volume.name.clone());
  }
  for config in &hir.configs {
    config_names.insert(config.name.clone());
  }
  for secret in &hir.secrets {
    secret_names.insert(secret.name.clone());
  }

  let volumes = hir
    .volumes
    .iter()
    .map(|volume| BoundVolume {
      name: volume.name.clone(),
    })
    .collect();
  let configs = hir
    .configs
    .iter()
    .map(|config| BoundConfig {
      name: config.name.clone(),
    })
    .collect();
  let secrets = hir
    .secrets
    .iter()
    .map(|secret| BoundSecret {
      name: secret.name.clone(),
    })
    .collect();
  let workloads = hir
    .services
    .iter()
    .map(|service| {
      bind_workload(
        service,
        &bindings,
        &names,
        &volume_names,
        &config_names,
        &secret_names,
        &endpoint_names,
      )
    })
    .collect::<Result<Vec<_>, _>>()?;
  let ingresses = hir
    .ingress_blocks
    .iter()
    .map(|ingress| bind_ingress(ingress, &bindings, &names, &endpoint_names))
    .collect::<Result<Vec<_>, _>>()?;

  Ok(BoundDocument {
    app_name: hir.app_name.clone(),
    lets: hir
      .lets
      .iter()
      .map(|item| BoundLet {
        name: item.name.to_string(),
        expr: item.expr.clone(),
      })
      .collect(),
    volumes,
    configs,
    secrets,
    workloads,
    ingresses,
  })
}

fn bind_workload(
  service: &warden_dsl_hir::HirService,
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
  volume_names: &HashSet<String>,
  config_names: &HashSet<String>,
  secret_names: &HashSet<String>,
  endpoint_names: &HashMap<String, Option<String>>,
) -> Result<BoundWorkload, BindError> {
  let local_endpoint = endpoint_name(&service.body);
  Ok(BoundWorkload {
    binding: service.binding.to_string(),
    name: service.name.clone(),
    kind: find_unary_invoke_text(&service.body, "kind"),
    runtime: find_unary_invoke_text(&service.body, "runtime"),
    image: find_string_first_arg(&service.body, "image"),
    endpoint_name: endpoint_name(&service.body),
    endpoint_protocol: endpoint_protocol(&service.body),
    service_port: endpoint_port(&service.name, &service.body)?,
    replicas: find_replicas(&service.body),
    depends_on: bind_depends_on(&service.name, &service.body, bindings, names)?,
    mounts: bind_mounts(&service.name, &service.body, volume_names)?,
    env: bind_env(
      &service.name,
      &service.body,
      bindings,
      names,
      config_names,
      secret_names,
      endpoint_names,
      local_endpoint.as_deref(),
    )?,
    resources: bind_resources(&service.body),
    health: bind_health(
      &service.name,
      &service.body,
      bindings,
      names,
      endpoint_names,
      local_endpoint.as_deref(),
    )?,
  })
}

fn bind_depends_on(
  workload: &str,
  body: &[StmtAst],
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
) -> Result<Vec<BoundWorkloadRef>, BindError> {
  let mut out = Vec::new();
  for stmt in body {
    let StmtAst::Invoke(invoke) = stmt else {
      continue;
    };
    if !path_is_single(&invoke.callee, "dependsOn") {
      continue;
    }
    for arg in &invoke.args {
      out.push(resolve_workload_ref(
        arg,
        bindings,
        names,
        &format!("workload {}", workload),
      )?);
    }
  }
  Ok(out)
}

fn bind_mounts(
  workload: &str,
  body: &[StmtAst],
  volume_names: &HashSet<String>,
) -> Result<Vec<BoundMount>, BindError> {
  let mut mounts = Vec::new();
  for stmt in body {
    let StmtAst::Invoke(invoke) = stmt else {
      continue;
    };
    if !path_is_single(&invoke.callee, "mount") {
      continue;
    }
    let Some(volume_expr) = invoke.args.first() else {
      continue;
    };
    let Some(target_expr) = invoke.args.get(1) else {
      return Err(BindError::InvalidMountTarget {
        workload: workload.to_string(),
      });
    };
    let volume = resolve_volume_ref(volume_expr, volume_names, &format!("workload {}", workload))?;
    let ExprAst::String(target) = target_expr else {
      return Err(BindError::InvalidMountTarget {
        workload: workload.to_string(),
      });
    };
    mounts.push(BoundMount {
      volume,
      target: target.clone(),
    });
  }
  Ok(mounts)
}

fn bind_env(
  workload: &str,
  body: &[StmtAst],
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
  config_names: &HashSet<String>,
  secret_names: &HashSet<String>,
  endpoint_names: &HashMap<String, Option<String>>,
  local_endpoint: Option<&str>,
) -> Result<Vec<BoundEnvVar>, BindError> {
  let mut vars = Vec::new();
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !path_is_single(&block.callee, "env") {
      continue;
    }
    for nested in &block.body {
      let StmtAst::Invoke(invoke) = nested else {
        continue;
      };
      if !path_is_single(&invoke.callee, "set") {
        continue;
      }
      let Some(ExprAst::String(name)) = invoke.args.first() else {
        return Err(BindError::InvalidEnvSet {
          workload: workload.to_string(),
        });
      };
      let Some(value_expr) = invoke.args.get(1) else {
        return Err(BindError::InvalidEnvSet {
          workload: workload.to_string(),
        });
      };
      let value = bind_env_value(
        workload,
        value_expr,
        bindings,
        names,
        config_names,
        secret_names,
        endpoint_names,
        local_endpoint,
      )?;
      vars.push(BoundEnvVar {
        name: name.clone(),
        value,
      });
    }
  }
  Ok(vars)
}

fn bind_resources(body: &[StmtAst]) -> Option<BoundResources> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !path_is_single(&block.callee, "resources") {
      continue;
    }
    let cpu = find_stringy_invoke_text(&block.body, "cpu");
    let memory = find_stringy_invoke_text(&block.body, "memory");
    return Some(BoundResources { cpu, memory });
  }
  None
}

fn bind_health(
  workload: &str,
  body: &[StmtAst],
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
  endpoint_names: &HashMap<String, Option<String>>,
  local_endpoint: Option<&str>,
) -> Result<Option<BoundHealth>, BindError> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !path_is_single(&block.callee, "health") {
      continue;
    }
    return Ok(Some(BoundHealth {
      readiness: bind_probe_block(
        &block.body,
        "readiness",
        bindings,
        names,
        endpoint_names,
        local_endpoint,
        workload,
      )?,
      liveness: bind_probe_block(
        &block.body,
        "liveness",
        bindings,
        names,
        endpoint_names,
        local_endpoint,
        workload,
      )?,
      startup: bind_probe_block(
        &block.body,
        "startup",
        bindings,
        names,
        endpoint_names,
        local_endpoint,
        workload,
      )?,
    }));
  }
  Ok(None)
}

fn bind_probe_block(
  body: &[StmtAst],
  stage: &str,
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
  endpoint_names: &HashMap<String, Option<String>>,
  local_endpoint: Option<&str>,
  workload: &str,
) -> Result<Option<BoundHttpProbe>, BindError> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !path_is_single(&block.callee, stage) {
      continue;
    }
    for nested in &block.body {
      let StmtAst::Invoke(invoke) = nested else {
        continue;
      };
      if !path_is_single(&invoke.callee, "http") {
        continue;
      }
      let Some(ExprAst::String(path)) = invoke.args.first() else {
        return Err(BindError::InvalidEnvValue {
          workload: workload.to_string(),
          raw: String::from("health.http"),
        });
      };
      let Some(endpoint_expr) = invoke.args.get(1) else {
        return Err(BindError::InvalidEnvValue {
          workload: workload.to_string(),
          raw: String::from("health.http"),
        });
      };
      return Ok(Some(BoundHttpProbe {
        path: path.clone(),
        endpoint: resolve_endpoint_ref_expr(
          endpoint_expr,
          bindings,
          names,
          endpoint_names,
          local_endpoint,
          &format!("workload {} {}", workload, stage),
        )?,
      }));
    }
  }
  Ok(None)
}

fn bind_ingress(
  ingress: &warden_dsl_hir::HirIngressBlock,
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
  endpoint_names: &HashMap<String, Option<String>>,
) -> Result<BoundIngress, BindError> {
  Ok(BoundIngress {
    name: ingress.name.clone(),
    host: find_string_first_arg(&ingress.body, "host"),
    routes: bind_routes(
      &ingress.name,
      &ingress.body,
      bindings,
      names,
      endpoint_names,
    )?,
  })
}

fn bind_routes(
  ingress_name: &str,
  body: &[StmtAst],
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
  endpoint_names: &HashMap<String, Option<String>>,
) -> Result<Vec<BoundIngressRoute>, BindError> {
  let mut routes = Vec::new();
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !path_is_single(&block.callee, "route") {
      continue;
    }
    let path = first_string_arg(&block.args);
    let port = find_string_first_arg(&block.body, "port");
    let backend = match find_first_invoke_arg(&block.body, "backend") {
      Some(arg @ ExprAst::Invocation { .. }) => Some(resolve_endpoint_ref_expr(
        arg,
        bindings,
        names,
        endpoint_names,
        None,
        &format!(
          "ingress {} route {}",
          ingress_name,
          path.as_deref().unwrap_or("<unknown>")
        ),
      )?),
      Some(arg) => Some(BoundEndpointRef {
        workload: resolve_workload_ref(
          arg,
          bindings,
          names,
          &format!(
            "ingress {} route {}",
            ingress_name,
            path.as_deref().unwrap_or("<unknown>")
          ),
        )?,
        endpoint: port.filter(|value| !value.trim().is_empty()),
      }),
      None => None,
    };
    routes.push(BoundIngressRoute { path, backend });
  }
  Ok(routes)
}

fn resolve_workload_ref(
  expr: &ExprAst,
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
  context: &str,
) -> Result<BoundWorkloadRef, BindError> {
  let raw = expr_to_text(expr);
  let token = match expr {
    ExprAst::Identifier(value) => value.as_str(),
    ExprAst::Path(path) if path.segments.len() == 2 => {
      let namespace = path.segments[0].as_str();
      if namespace != "services" && namespace != "workloads" {
        return Err(BindError::InvalidWorkloadRef {
          raw,
          context: context.to_string(),
        });
      }
      path.segments[1].as_str()
    }
    ExprAst::Path(path) if path.segments.len() == 1 => path.segments[0].as_str(),
    _ => {
      return Err(BindError::InvalidWorkloadRef {
        raw,
        context: context.to_string(),
      });
    }
  };

  if let Some(name) = bindings.get(token) {
    return Ok(BoundWorkloadRef { name: name.clone() });
  }
  if names.contains(token) {
    return Ok(BoundWorkloadRef {
      name: token.to_string(),
    });
  }
  Err(BindError::UnknownWorkloadRef {
    raw: token.to_string(),
    context: context.to_string(),
  })
}

fn resolve_volume_ref(
  expr: &ExprAst,
  names: &HashSet<String>,
  context: &str,
) -> Result<BoundVolumeRef, BindError> {
  let raw = expr_to_text(expr);
  let token = match expr {
    ExprAst::Path(path) if path.segments.len() == 2 => {
      let namespace = path.segments[0].as_str();
      if namespace != "volumes" {
        return Err(BindError::InvalidVolumeRef {
          raw,
          context: context.to_string(),
        });
      }
      path.segments[1].as_str()
    }
    _ => {
      return Err(BindError::InvalidVolumeRef {
        raw,
        context: context.to_string(),
      });
    }
  };

  if names.contains(token) {
    return Ok(BoundVolumeRef {
      name: token.to_string(),
    });
  }
  Err(BindError::UnknownVolumeRef {
    raw: token.to_string(),
    context: context.to_string(),
  })
}

fn resolve_config_ref(
  expr: &ExprAst,
  names: &HashSet<String>,
  context: &str,
) -> Result<BoundConfigRef, BindError> {
  let raw = expr_to_text(expr);
  let token = match expr {
    ExprAst::Path(path) if path.segments.len() == 2 => {
      if path.segments[0].as_str() != "configs" {
        return Err(BindError::InvalidConfigRef {
          raw,
          context: context.to_string(),
        });
      }
      path.segments[1].as_str()
    }
    _ => {
      return Err(BindError::InvalidConfigRef {
        raw,
        context: context.to_string(),
      });
    }
  };

  if names.contains(token) {
    return Ok(BoundConfigRef {
      name: token.to_string(),
    });
  }
  Err(BindError::UnknownConfigRef {
    raw: token.to_string(),
    context: context.to_string(),
  })
}

fn resolve_secret_ref(
  expr: &ExprAst,
  names: &HashSet<String>,
  context: &str,
) -> Result<BoundSecretRef, BindError> {
  let raw = expr_to_text(expr);
  let token = match expr {
    ExprAst::Path(path) if path.segments.len() == 2 => {
      if path.segments[0].as_str() != "secrets" {
        return Err(BindError::InvalidSecretRef {
          raw,
          context: context.to_string(),
        });
      }
      path.segments[1].as_str()
    }
    _ => {
      return Err(BindError::InvalidSecretRef {
        raw,
        context: context.to_string(),
      });
    }
  };

  if names.contains(token) {
    return Ok(BoundSecretRef {
      name: token.to_string(),
    });
  }
  Err(BindError::UnknownSecretRef {
    raw: token.to_string(),
    context: context.to_string(),
  })
}

fn bind_env_value(
  workload: &str,
  expr: &ExprAst,
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
  config_names: &HashSet<String>,
  secret_names: &HashSet<String>,
  endpoint_names: &HashMap<String, Option<String>>,
  local_endpoint: Option<&str>,
) -> Result<BoundEnvValue, BindError> {
  let context = format!("workload {}", workload);
  match expr {
    ExprAst::String(value) => Ok(BoundEnvValue::String(value.clone())),
    ExprAst::Path(path)
      if path
        .segments
        .first()
        .is_some_and(|value| value == "configs") =>
    {
      Ok(BoundEnvValue::ConfigRef(resolve_config_ref(
        expr,
        config_names,
        &context,
      )?))
    }
    ExprAst::Path(path)
      if path
        .segments
        .first()
        .is_some_and(|value| value == "secrets") =>
    {
      Ok(BoundEnvValue::SecretRef(resolve_secret_ref(
        expr,
        secret_names,
        &context,
      )?))
    }
    ExprAst::Invocation { callee, args } => {
      let endpoint = resolve_endpoint_ref(
        expr,
        callee,
        args,
        bindings,
        names,
        endpoint_names,
        local_endpoint,
        &context,
      )?;
      Ok(BoundEnvValue::EndpointRef(endpoint))
    }
    _ => Err(BindError::InvalidEnvValue {
      workload: workload.to_string(),
      raw: expr_to_text(expr),
    }),
  }
}

fn resolve_endpoint_ref_expr(
  expr: &ExprAst,
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
  endpoint_names: &HashMap<String, Option<String>>,
  local_endpoint: Option<&str>,
  context: &str,
) -> Result<BoundEndpointRef, BindError> {
  let ExprAst::Invocation { callee, args } = expr else {
    return Err(BindError::InvalidEndpointRef {
      raw: expr_to_text(expr),
      context: context.to_string(),
    });
  };
  resolve_endpoint_ref(
    expr,
    callee,
    args,
    bindings,
    names,
    endpoint_names,
    local_endpoint,
    context,
  )
}

fn resolve_endpoint_ref(
  expr: &ExprAst,
  callee: &PathAst,
  args: &[ExprAst],
  bindings: &HashMap<String, String>,
  names: &HashSet<String>,
  endpoint_names: &HashMap<String, Option<String>>,
  local_endpoint: Option<&str>,
  context: &str,
) -> Result<BoundEndpointRef, BindError> {
  let raw = expr_to_text(expr);
  let Some(ExprAst::String(endpoint_name)) = args.first() else {
    return Err(BindError::InvalidEndpointRef {
      raw,
      context: context.to_string(),
    });
  };
  let workload = match callee.segments.as_slice() {
    [single] if single.as_str() == "endpoint" => BoundWorkloadRef {
      name: context
        .strip_prefix("workload ")
        .unwrap_or(context)
        .to_string(),
    },
    [target, last] if last.as_str() == "endpoint" => resolve_workload_ref(
      &ExprAst::Path(PathAst {
        segments: vec![target.clone()],
      }),
      bindings,
      names,
      context,
    )?,
    [namespace, target, last] if last.as_str() == "endpoint" => resolve_workload_ref(
      &ExprAst::Path(PathAst {
        segments: vec![namespace.clone(), target.clone()],
      }),
      bindings,
      names,
      context,
    )?,
    _ => {
      return Err(BindError::InvalidEndpointRef {
        raw,
        context: context.to_string(),
      });
    }
  };
  let known_endpoint = if workload.name == context.strip_prefix("workload ").unwrap_or(context) {
    local_endpoint.map(ToString::to_string)
  } else {
    endpoint_names.get(&workload.name).cloned().unwrap_or(None)
  };
  if let Some(known) = known_endpoint
    && known != *endpoint_name
  {
    return Err(BindError::UnknownEndpointRef {
      raw,
      context: context.to_string(),
    });
  }
  Ok(BoundEndpointRef {
    workload,
    endpoint: Some(endpoint_name.clone()),
  })
}

fn path_is_single(path: &PathAst, want: &str) -> bool {
  path.segments.len() == 1 && path.segments[0].as_str() == want
}

fn first_string_arg(args: &[ExprAst]) -> Option<String> {
  match args.first()? {
    ExprAst::String(value) => Some(value.clone()),
    _ => None,
  }
}

fn find_first_invoke_arg<'a>(body: &'a [StmtAst], name: &str) -> Option<&'a ExprAst> {
  for stmt in body {
    let StmtAst::Invoke(invoke) = stmt else {
      continue;
    };
    if path_is_single(&invoke.callee, name) {
      return invoke.args.first();
    }
  }
  None
}

fn find_string_first_arg(body: &[StmtAst], name: &str) -> Option<String> {
  match find_first_invoke_arg(body, name)? {
    ExprAst::String(value) => Some(value.clone()),
    _ => None,
  }
}

fn find_unary_invoke_text(body: &[StmtAst], name: &str) -> Option<String> {
  find_first_invoke_arg(body, name).map(expr_to_text)
}

fn find_stringy_invoke_text(body: &[StmtAst], name: &str) -> Option<String> {
  find_first_invoke_arg(body, name).map(expr_to_text)
}

fn find_replicas(body: &[StmtAst]) -> Option<String> {
  find_first_invoke_arg(body, "replicas").map(expr_to_text)
}

fn endpoint_name(body: &[StmtAst]) -> Option<String> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !path_is_single(&block.callee, "endpoint") {
      continue;
    }
    if let Some(ExprAst::String(name)) = block.args.first() {
      return Some(name.clone());
    }
  }
  legacy_expose_name(body)
}

fn legacy_expose_name(body: &[StmtAst]) -> Option<String> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !path_is_single(&block.callee, "expose") {
      continue;
    }
    if let Some(ExprAst::String(name)) = block.args.first() {
      return Some(name.clone());
    }
  }
  None
}

fn endpoint_port(workload: &str, body: &[StmtAst]) -> Result<Option<u16>, BindError> {
  let mut saw_endpoint = false;
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !path_is_single(&block.callee, "endpoint") {
      continue;
    }
    saw_endpoint = true;
    for nested in &block.body {
      let StmtAst::Invoke(invoke) = nested else {
        continue;
      };
      if !path_is_single(&invoke.callee, "port") {
        continue;
      }
      if let Some(arg) = invoke.args.first() {
        return match arg {
          ExprAst::Integer(port) => Ok(u16::try_from(*port).ok()),
          _ => Err(BindError::InvalidEndpointPort {
            workload: workload.to_string(),
            raw: expr_to_text(arg),
          }),
        };
      }
    }
  }
  if saw_endpoint {
    return Ok(None);
  }
  legacy_expose_container_port(workload, body)
}

fn endpoint_protocol(body: &[StmtAst]) -> Option<String> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !path_is_single(&block.callee, "endpoint") {
      continue;
    }
    for nested in &block.body {
      let StmtAst::Invoke(invoke) = nested else {
        continue;
      };
      if !path_is_single(&invoke.callee, "protocol") {
        continue;
      }
      return invoke.args.first().map(expr_to_text);
    }
  }
  None
}

fn legacy_expose_container_port(
  workload: &str,
  body: &[StmtAst],
) -> Result<Option<u16>, BindError> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !path_is_single(&block.callee, "expose") {
      continue;
    }
    for nested in &block.body {
      let StmtAst::Invoke(invoke) = nested else {
        continue;
      };
      if !path_is_single(&invoke.callee, "container") {
        continue;
      }
      if let Some(arg) = invoke.args.first() {
        return match arg {
          ExprAst::Integer(port) => Ok(u16::try_from(*port).ok()),
          _ => Err(BindError::InvalidContainerPort {
            workload: workload.to_string(),
            raw: expr_to_text(arg),
          }),
        };
      }
    }
  }
  Ok(None)
}

fn expr_to_text(expr: &ExprAst) -> String {
  match expr {
    ExprAst::String(value) => value.clone(),
    ExprAst::Integer(value) => value.to_string(),
    ExprAst::Identifier(value) => value.to_string(),
    ExprAst::Path(path) => path
      .segments
      .iter()
      .map(|seg| seg.as_str())
      .collect::<Vec<_>>()
      .join("."),
    ExprAst::MemberNumber { value, unit } => format!("{}.{}", value, unit),
    ExprAst::IfEq {
      left,
      right,
      then_expr,
      else_expr,
    } => format!(
      "if {} == {:?} then {} else {}",
      left,
      right,
      expr_to_text(then_expr),
      expr_to_text(else_expr)
    ),
    ExprAst::Invocation { callee, args } => {
      let mut out = callee
        .segments
        .iter()
        .map(|seg| seg.as_str())
        .collect::<Vec<_>>()
        .join(".");
      out.push('(');
      for (idx, arg) in args.iter().enumerate() {
        if idx > 0 {
          out.push_str(", ");
        }
        out.push_str(&expr_to_text(arg));
      }
      out.push(')');
      out
    }
  }
}

#[cfg(test)]
mod tests {
  use super::*;
  use warden_dsl_hir::lower as lower_hir;
  use warden_dsl_parser::parse;

  #[test]
  fn binds_depends_on_and_ingress_backend_refs() {
    let raw = r#"
app("mall") {
  config("appConfig") {}
  secret("dbPassword") {}
  volume("redisData") {}
  workload("redis") {
    kind(stateful)
    runtime(container)
    endpoint("redis") { port(6379) protocol(tcp) }
  }
  workload("gateway") {
    kind(service)
    runtime(containerd)
    dependsOn(workloads.redis)
    mount(volumes.redisData, "/data")
    env {
      set("APP_CONFIG", configs.appConfig)
      set("DB_PASSWORD", secrets.dbPassword)
      set("REDIS_ADDR", workloads.redis.endpoint("redis"))
      set("MODE", "prod")
    }
    endpoint("http") { port(8080) protocol(http) }
    resources {
      cpu(500.milliCpu)
      memory(512.mebi)
    }
    health {
      readiness { http("/ready", endpoint("http")) }
      liveness { http("/live", workloads.gateway.endpoint("http")) }
    }
  }
  ingress("public") {
    route("/") { backend(workloads.gateway.endpoint("http")) }
  }
}
"#;
    let ast = parse(raw).unwrap();
    let hir = lower_hir(&ast).unwrap();
    let bound = bind(&hir).unwrap();

    assert_eq!(bound.configs[0].name, "appConfig");
    assert_eq!(bound.secrets[0].name, "dbPassword");
    assert_eq!(bound.volumes[0].name, "redisData");
    assert_eq!(bound.workloads[1].depends_on[0].name, "redis");
    assert_eq!(bound.workloads[1].mounts[0].volume.name, "redisData");
    assert_eq!(bound.workloads[1].mounts[0].target, "/data");
    assert_eq!(bound.workloads[1].env.len(), 4);
    assert_eq!(
      bound.workloads[1]
        .resources
        .as_ref()
        .and_then(|value| value.cpu.as_deref()),
      Some("500.milliCpu")
    );
    assert_eq!(
      bound.workloads[1]
        .resources
        .as_ref()
        .and_then(|value| value.memory.as_deref()),
      Some("512.mebi")
    );
    assert_eq!(
      bound.workloads[1]
        .health
        .as_ref()
        .and_then(|value| value.readiness.as_ref())
        .map(|value| value.path.as_str()),
      Some("/ready")
    );
    assert_eq!(
      bound.workloads[1]
        .health
        .as_ref()
        .and_then(|value| value.liveness.as_ref())
        .and_then(|value| value.endpoint.endpoint.as_deref()),
      Some("http")
    );
    assert_eq!(bound.workloads[0].kind.as_deref(), Some("stateful"));
    assert_eq!(bound.workloads[0].endpoint_protocol.as_deref(), Some("tcp"));
    assert!(matches!(
      bound.workloads[1].env[0].value,
      BoundEnvValue::ConfigRef(_)
    ));
    assert!(matches!(
      bound.workloads[1].env[1].value,
      BoundEnvValue::SecretRef(_)
    ));
    assert!(matches!(
      bound.workloads[1].env[2].value,
      BoundEnvValue::EndpointRef(_)
    ));
    assert!(matches!(
      bound.workloads[1].env[3].value,
      BoundEnvValue::String(_)
    ));
    assert_eq!(
      bound.ingresses[0].routes[0]
        .backend
        .as_ref()
        .unwrap()
        .workload
        .name,
      "gateway"
    );
    assert_eq!(
      bound.ingresses[0].routes[0]
        .backend
        .as_ref()
        .unwrap()
        .endpoint
        .as_deref(),
      Some("http")
    );
  }

  #[test]
  fn rejects_unknown_workload_refs() {
    let raw = r#"
app("mall") {
  services {
    val gateway = create("gateway") {
      dependsOn(redis)
    }
  }
}
"#;
    let ast = parse(raw).unwrap();
    let hir = lower_hir(&ast).unwrap();
    let err = bind(&hir).unwrap_err();
    assert!(err.to_string().contains("unknown workload reference"));
  }

  #[test]
  fn keeps_legacy_expose_compatibility() {
    let raw = r#"
app("mall") {
  services {
    val redis = create("redis") {
      expose("redis") { container(6379) }
    }
  }
}
"#;
    let ast = parse(raw).unwrap();
    let hir = lower_hir(&ast).unwrap();
    let bound = bind(&hir).unwrap();
    assert_eq!(bound.workloads[0].endpoint_name.as_deref(), Some("redis"));
    assert_eq!(bound.workloads[0].service_port, Some(6379));
  }

  #[test]
  fn rejects_unknown_volume_refs() {
    let raw = r#"
app("mall") {
  workload("gateway") {
    mount(volumes.redisData, "/data")
  }
}
"#;
    let ast = parse(raw).unwrap();
    let hir = lower_hir(&ast).unwrap();
    let err = bind(&hir).unwrap_err();
    assert!(err.to_string().contains("unknown volume reference"));
  }

  #[test]
  fn rejects_unknown_config_refs() {
    let raw = r#"
app("mall") {
  workload("gateway") {
    env { set("APP_CONFIG", configs.appConfig) }
  }
}
"#;
    let ast = parse(raw).unwrap();
    let hir = lower_hir(&ast).unwrap();
    let err = bind(&hir).unwrap_err();
    assert!(err.to_string().contains("unknown config reference"));
  }
}
