use std::collections::{HashMap, HashSet};
use warden_dsl_ast::ExprAst;
use warden_dsl_binder::{
  BoundDocument, BoundEnvValue, BoundIngress, BoundIngressRoute, BoundWorkload, bind,
};
use warden_dsl_hir::HirDocument;
use warden_dsl_ir::{IrApplication, IrIngress, IrRoute, IrWorkload};
mod types;

pub use types::*;

pub fn string_lets(hir: &HirDocument) -> Result<HashMap<String, String>, CanonicalLowerError> {
  let mut map = HashMap::new();
  for binding in &hir.lets {
    match &binding.expr {
      ExprAst::String(value) => {
        map.insert(binding.name.to_string(), value.clone());
      }
      _ => {
        return Err(CanonicalLowerError::NonStringLet(binding.name.to_string()));
      }
    }
  }
  Ok(map)
}

pub fn lower(hir: &HirDocument) -> Result<CanonicalApplication, CanonicalLowerError> {
  let bound = bind(hir).map_err(|err| CanonicalLowerError::Bind(err.to_string()))?;
  lower_bound(&bound)
}

pub fn lower_ir(
  ir: &IrApplication,
  lets: &HashMap<String, String>,
) -> Result<CanonicalApplication, CanonicalLowerError> {
  let alias_to_name = ir
    .workloads
    .iter()
    .map(|workload| (workload.binding.clone(), workload.name.clone()))
    .collect::<HashMap<_, _>>();
  let config_names = ir
    .configs
    .iter()
    .map(|config| config.name.clone())
    .collect::<HashSet<_>>();
  let secret_names = ir
    .secrets
    .iter()
    .map(|secret| secret.name.clone())
    .collect::<HashSet<_>>();

  let workloads = ir
    .workloads
    .iter()
    .map(|workload| lower_workload(workload, lets, &alias_to_name, &config_names, &secret_names))
    .collect::<Result<Vec<_>, _>>()?;
  let volumes = ir
    .volumes
    .iter()
    .map(|volume| CanonicalVolume {
      name: volume.name.clone(),
    })
    .collect::<Vec<_>>();
  let configs = ir
    .configs
    .iter()
    .map(|config| CanonicalConfig {
      name: config.name.clone(),
    })
    .collect::<Vec<_>>();
  let secrets = ir
    .secrets
    .iter()
    .map(|secret| CanonicalSecret {
      name: secret.name.clone(),
    })
    .collect::<Vec<_>>();

  let ingresses = ir
    .ingress
    .iter()
    .map(|ingress| lower_ingress(ingress, &alias_to_name))
    .collect::<Vec<_>>();

  Ok(CanonicalApplication {
    metadata: CanonicalMetadata {
      name: ir.app_name.clone(),
      namespace: default_namespace(),
    },
    workloads,
    configs,
    secrets,
    volumes,
    ingresses,
  })
}

pub fn lower_bound(bound: &BoundDocument) -> Result<CanonicalApplication, CanonicalLowerError> {
  let lets = bound
    .lets
    .iter()
    .map(|binding| match &binding.expr {
      ExprAst::String(value) => Ok((binding.name.clone(), value.clone())),
      _ => Err(CanonicalLowerError::NonStringLet(binding.name.clone())),
    })
    .collect::<Result<HashMap<_, _>, _>>()?;
  let workloads = bound
    .workloads
    .iter()
    .map(|workload| lower_bound_workload(workload, &lets))
    .collect::<Result<Vec<_>, _>>()?;
  let volumes = bound
    .volumes
    .iter()
    .map(|volume| CanonicalVolume {
      name: volume.name.clone(),
    })
    .collect::<Vec<_>>();
  let configs = bound
    .configs
    .iter()
    .map(|config| CanonicalConfig {
      name: config.name.clone(),
    })
    .collect::<Vec<_>>();
  let secrets = bound
    .secrets
    .iter()
    .map(|secret| CanonicalSecret {
      name: secret.name.clone(),
    })
    .collect::<Vec<_>>();
  let ingresses = bound.ingresses.iter().map(lower_bound_ingress).collect();

  Ok(CanonicalApplication {
    metadata: CanonicalMetadata {
      name: bound.app_name.clone(),
      namespace: default_namespace(),
    },
    workloads,
    configs,
    secrets,
    volumes,
    ingresses,
  })
}

pub fn compile_apply_output(app: &CanonicalApplication) -> CanonicalApplyOutput {
  CanonicalApplyOutput {
    application: app.metadata.name.clone(),
    namespace: app.metadata.namespace.clone(),
    ingress_routes: app
      .ingresses
      .iter()
      .flat_map(|ingress| compile_ingress_route_specs(app, ingress))
      .collect::<Vec<_>>(),
  }
}

fn lower_workload(
  workload: &IrWorkload,
  lets: &HashMap<String, String>,
  alias_to_name: &HashMap<String, String>,
  config_names: &HashSet<String>,
  secret_names: &HashSet<String>,
) -> Result<CanonicalWorkload, CanonicalLowerError> {
  let kind = normalize_workload_kind(workload.kind.as_deref().unwrap_or("service"))?;
  let runtime = normalize_runtime(workload.runtime.as_deref().unwrap_or("docker"))?;
  let image = workload
    .image
    .as_deref()
    .map(|value| interpolate_template(value, lets))
    .transpose()?;
  let replicas = workload
    .replicas
    .as_deref()
    .map(|expr| eval_replicas_expression(expr, lets))
    .transpose()?;
  let depends_on = workload
    .depends_on
    .iter()
    .map(|dep| CanonicalWorkloadRef {
      name: alias_to_name
        .get(last_segment(dep))
        .cloned()
        .unwrap_or_else(|| dep.clone()),
    })
    .collect::<Vec<_>>();
  let endpoints = match workload.service_port {
    Some(port) => vec![CanonicalEndpoint {
      name: workload
        .endpoint_name
        .clone()
        .unwrap_or_else(|| String::from("default")),
      port,
      protocol: normalize_endpoint_protocol(
        workload.endpoint_protocol.as_deref(),
        workload.endpoint_name.as_deref(),
      )?,
    }],
    None => Vec::new(),
  };
  let mounts = workload
    .mounts
    .iter()
    .map(|mount| CanonicalMount {
      volume: CanonicalVolumeRef {
        name: last_segment(&mount.volume).to_string(),
      },
      target: mount.target.clone(),
    })
    .collect::<Vec<_>>();
  let env = workload
    .env
    .iter()
    .map(|entry| {
      Ok(CanonicalEnvVar {
        name: entry.name.clone(),
        value: lower_ir_env_value(
          &entry.value,
          &workload.name,
          workload.endpoint_name.as_deref(),
          alias_to_name,
          config_names,
          secret_names,
        )?,
      })
    })
    .collect::<Result<Vec<_>, _>>()?;
  let resources = workload
    .resources
    .as_ref()
    .map(lower_ir_resources)
    .transpose()?;
  let health = workload
    .health
    .as_ref()
    .map(|value| lower_ir_health(value, &workload.name, alias_to_name))
    .transpose()?;

  Ok(CanonicalWorkload {
    name: workload.name.clone(),
    kind,
    runtime,
    run: CanonicalRun {
      image,
      env,
      resources,
    },
    replicas,
    depends_on,
    endpoints,
    mounts,
    health,
  })
}

fn lower_bound_workload(
  workload: &BoundWorkload,
  lets: &HashMap<String, String>,
) -> Result<CanonicalWorkload, CanonicalLowerError> {
  let kind = normalize_workload_kind(workload.kind.as_deref().unwrap_or("service"))?;
  let runtime = normalize_runtime(workload.runtime.as_deref().unwrap_or("docker"))?;
  let image = workload
    .image
    .as_deref()
    .map(|value| interpolate_template(value, lets))
    .transpose()?;
  let replicas = workload
    .replicas
    .as_deref()
    .map(|expr| eval_replicas_expression(expr, lets))
    .transpose()?;
  let depends_on = workload
    .depends_on
    .iter()
    .map(|dep| CanonicalWorkloadRef {
      name: dep.name.clone(),
    })
    .collect::<Vec<_>>();
  let endpoints = match workload.service_port {
    Some(port) => vec![CanonicalEndpoint {
      name: workload
        .endpoint_name
        .clone()
        .unwrap_or_else(|| String::from("default")),
      port,
      protocol: normalize_endpoint_protocol(
        workload.endpoint_protocol.as_deref(),
        workload.endpoint_name.as_deref(),
      )?,
    }],
    None => Vec::new(),
  };
  let mounts = workload
    .mounts
    .iter()
    .map(|mount| CanonicalMount {
      volume: CanonicalVolumeRef {
        name: mount.volume.name.clone(),
      },
      target: mount.target.clone(),
    })
    .collect::<Vec<_>>();
  let env = workload
    .env
    .iter()
    .map(|entry| CanonicalEnvVar {
      name: entry.name.clone(),
      value: lower_bound_env_value(&entry.value),
    })
    .collect::<Vec<_>>();
  let resources = workload
    .resources
    .as_ref()
    .map(lower_bound_resources)
    .transpose()?;
  let health = workload
    .health
    .as_ref()
    .map(lower_bound_health)
    .transpose()?;

  Ok(CanonicalWorkload {
    name: workload.name.clone(),
    kind,
    runtime,
    run: CanonicalRun {
      image,
      env,
      resources,
    },
    replicas,
    depends_on,
    endpoints,
    mounts,
    health,
  })
}

fn lower_ingress(ingress: &IrIngress, alias_to_name: &HashMap<String, String>) -> CanonicalIngress {
  let routes = ingress
    .routes
    .iter()
    .filter_map(|route| lower_route(route, alias_to_name))
    .collect::<Vec<_>>();
  CanonicalIngress {
    name: ingress.name.clone(),
    host: ingress
      .host
      .clone()
      .unwrap_or_else(|| String::from("warden.local")),
    routes,
  }
}

fn lower_bound_ingress(ingress: &BoundIngress) -> CanonicalIngress {
  CanonicalIngress {
    name: ingress.name.clone(),
    host: ingress
      .host
      .clone()
      .unwrap_or_else(|| String::from("warden.local")),
    routes: ingress
      .routes
      .iter()
      .filter_map(lower_bound_route)
      .collect::<Vec<_>>(),
  }
}

fn compile_ingress_route_specs(
  app: &CanonicalApplication,
  ingress: &CanonicalIngress,
) -> Vec<CanonicalIngressRouteSpec> {
  let host = ingress.host.trim();
  let host = if host.is_empty() {
    "warden.local"
  } else {
    host
  };
  ingress
    .routes
    .iter()
    .map(|route| CanonicalIngressRouteSpec {
      id: stable_ingress_route_id(app, ingress, route),
      ingress_name: ingress.name.clone(),
      protocol: String::from("http"),
      host: host.to_string(),
      path_prefix: ensure_route_path(&route.path),
      listen_port: 8088,
      backend: route.backend.clone(),
      dns_enabled: true,
      dns_ttl: 60,
    })
    .collect::<Vec<_>>()
}

fn lower_route(
  route: &IrRoute,
  alias_to_name: &HashMap<String, String>,
) -> Option<CanonicalIngressRoute> {
  let backend = route.backend.as_ref()?;
  let workload = alias_to_name
    .get(last_segment(&backend.workload))
    .cloned()
    .unwrap_or_else(|| last_segment(&backend.workload).to_string());
  Some(CanonicalIngressRoute {
    path: route.path.clone().unwrap_or_else(|| String::from("/")),
    backend: CanonicalEndpointRef {
      workload,
      endpoint: backend
        .endpoint
        .clone()
        .filter(|value| !value.trim().is_empty()),
    },
  })
}

fn lower_bound_route(route: &BoundIngressRoute) -> Option<CanonicalIngressRoute> {
  let backend = route.backend.as_ref()?;
  Some(CanonicalIngressRoute {
    path: route.path.clone().unwrap_or_else(|| String::from("/")),
    backend: CanonicalEndpointRef {
      workload: backend.workload.name.clone(),
      endpoint: backend.endpoint.clone(),
    },
  })
}

fn lower_ir_env_value(
  expr: &ExprAst,
  current_workload: &str,
  local_endpoint: Option<&str>,
  alias_to_name: &HashMap<String, String>,
  config_names: &HashSet<String>,
  secret_names: &HashSet<String>,
) -> Result<CanonicalEnvValue, CanonicalLowerError> {
  match expr {
    ExprAst::String(value) => Ok(CanonicalEnvValue::String(value.clone())),
    ExprAst::Path(path) if path.segments.len() == 2 && path.segments[0].as_str() == "configs" => {
      let name = path.segments[1].to_string();
      let _ = config_names.contains(&name);
      Ok(CanonicalEnvValue::ConfigRef(CanonicalConfigRef { name }))
    }
    ExprAst::Path(path) if path.segments.len() == 2 && path.segments[0].as_str() == "secrets" => {
      let name = path.segments[1].to_string();
      let _ = secret_names.contains(&name);
      Ok(CanonicalEnvValue::SecretRef(CanonicalSecretRef { name }))
    }
    ExprAst::Invocation { callee, args } => {
      let Some(ExprAst::String(endpoint)) = args.first() else {
        return Err(CanonicalLowerError::InvalidEnvValue(expr_to_text(expr)));
      };
      let workload = match callee.segments.as_slice() {
        [single] if single.as_str() == "endpoint" => current_workload.to_string(),
        [target, last] if last.as_str() == "endpoint" => alias_to_name
          .get(target.as_str())
          .cloned()
          .unwrap_or_else(|| target.to_string()),
        [namespace, target, last] if last.as_str() == "endpoint" => {
          if namespace.as_str() != "services" && namespace.as_str() != "workloads" {
            return Err(CanonicalLowerError::InvalidEnvValue(expr_to_text(expr)));
          }
          alias_to_name
            .get(target.as_str())
            .cloned()
            .unwrap_or_else(|| target.to_string())
        }
        _ => return Err(CanonicalLowerError::InvalidEnvValue(expr_to_text(expr))),
      };
      let endpoint = if workload == current_workload {
        Some(endpoint.clone()).filter(|value| {
          local_endpoint
            .map(|known| known == value.as_str())
            .unwrap_or(true)
        })
      } else {
        Some(endpoint.clone())
      };
      Ok(CanonicalEnvValue::EndpointRef(CanonicalEndpointRef {
        workload,
        endpoint,
      }))
    }
    _ => Err(CanonicalLowerError::InvalidEnvValue(expr_to_text(expr))),
  }
}

fn lower_bound_env_value(value: &BoundEnvValue) -> CanonicalEnvValue {
  match value {
    BoundEnvValue::String(value) => CanonicalEnvValue::String(value.clone()),
    BoundEnvValue::ConfigRef(value) => CanonicalEnvValue::ConfigRef(CanonicalConfigRef {
      name: value.name.clone(),
    }),
    BoundEnvValue::SecretRef(value) => CanonicalEnvValue::SecretRef(CanonicalSecretRef {
      name: value.name.clone(),
    }),
    BoundEnvValue::EndpointRef(value) => CanonicalEnvValue::EndpointRef(CanonicalEndpointRef {
      workload: value.workload.name.clone(),
      endpoint: value.endpoint.clone(),
    }),
  }
}

fn lower_ir_resources(
  resources: &warden_dsl_ir::IrResources,
) -> Result<CanonicalResources, CanonicalLowerError> {
  Ok(CanonicalResources {
    cpu_millis: resources
      .cpu
      .as_deref()
      .map(parse_cpu_resource)
      .transpose()?,
    memory_bytes: resources
      .memory
      .as_deref()
      .map(parse_memory_resource)
      .transpose()?,
  })
}

fn lower_bound_resources(
  resources: &warden_dsl_binder::BoundResources,
) -> Result<CanonicalResources, CanonicalLowerError> {
  Ok(CanonicalResources {
    cpu_millis: resources
      .cpu
      .as_deref()
      .map(parse_cpu_resource)
      .transpose()?,
    memory_bytes: resources
      .memory
      .as_deref()
      .map(parse_memory_resource)
      .transpose()?,
  })
}

fn lower_ir_health(
  health: &warden_dsl_ir::IrHealth,
  current_workload: &str,
  alias_to_name: &HashMap<String, String>,
) -> Result<CanonicalHealth, CanonicalLowerError> {
  Ok(CanonicalHealth {
    readiness: health
      .readiness
      .as_ref()
      .map(|probe| lower_ir_probe(probe, current_workload, alias_to_name))
      .transpose()?,
    liveness: health
      .liveness
      .as_ref()
      .map(|probe| lower_ir_probe(probe, current_workload, alias_to_name))
      .transpose()?,
    startup: health
      .startup
      .as_ref()
      .map(|probe| lower_ir_probe(probe, current_workload, alias_to_name))
      .transpose()?,
  })
}

fn lower_bound_health(
  health: &warden_dsl_binder::BoundHealth,
) -> Result<CanonicalHealth, CanonicalLowerError> {
  Ok(CanonicalHealth {
    readiness: health
      .readiness
      .as_ref()
      .map(lower_bound_probe)
      .transpose()?,
    liveness: health
      .liveness
      .as_ref()
      .map(lower_bound_probe)
      .transpose()?,
    startup: health.startup.as_ref().map(lower_bound_probe).transpose()?,
  })
}

fn lower_ir_probe(
  probe: &warden_dsl_ir::IrHttpProbe,
  current_workload: &str,
  alias_to_name: &HashMap<String, String>,
) -> Result<CanonicalHttpProbe, CanonicalLowerError> {
  let (backend_raw, endpoint_name) = split_endpoint_ref(&probe.endpoint)?;
  let workload = if backend_raw.is_empty() {
    current_workload.to_string()
  } else {
    alias_to_name
      .get(last_segment(backend_raw))
      .cloned()
      .unwrap_or_else(|| last_segment(backend_raw).to_string())
  };
  Ok(CanonicalHttpProbe {
    path: probe.path.clone(),
    endpoint: CanonicalEndpointRef {
      workload,
      endpoint: Some(endpoint_name),
    },
  })
}

fn lower_bound_probe(
  probe: &warden_dsl_binder::BoundHttpProbe,
) -> Result<CanonicalHttpProbe, CanonicalLowerError> {
  Ok(CanonicalHttpProbe {
    path: probe.path.clone(),
    endpoint: CanonicalEndpointRef {
      workload: probe.endpoint.workload.name.clone(),
      endpoint: probe.endpoint.endpoint.clone(),
    },
  })
}

fn split_endpoint_ref(raw: &str) -> Result<(&str, String), CanonicalLowerError> {
  if let Some(endpoint_ref) = raw.strip_prefix("endpoint(") {
    let endpoint_name = endpoint_ref.trim_end_matches(')').trim().to_string();
    return Ok(("", endpoint_name));
  }
  let Some(prefix) = raw.strip_suffix(')') else {
    return Err(CanonicalLowerError::InvalidEnvValue(raw.to_string()));
  };
  let Some((workload_ref, endpoint_ref)) = prefix.rsplit_once(".endpoint(") else {
    return Err(CanonicalLowerError::InvalidEnvValue(raw.to_string()));
  };
  let endpoint_name = endpoint_ref.trim().trim_matches('"').to_string();
  Ok((workload_ref, endpoint_name))
}

fn parse_cpu_resource(raw: &str) -> Result<u32, CanonicalLowerError> {
  let value = raw.trim();
  if let Ok(cores) = value.parse::<u32>() {
    return cores
      .checked_mul(1000)
      .ok_or_else(|| CanonicalLowerError::InvalidCpuResource(value.to_string()));
  }
  if let Some((number, unit)) = value.split_once('.') {
    let parsed = number
      .trim()
      .parse::<u32>()
      .map_err(|_| CanonicalLowerError::InvalidCpuResource(value.to_string()))?;
    return match unit.trim() {
      "milli" | "milliCpu" => Ok(parsed),
      "cpu" => parsed
        .checked_mul(1000)
        .ok_or_else(|| CanonicalLowerError::InvalidCpuResource(value.to_string())),
      _ => Err(CanonicalLowerError::InvalidCpuResource(value.to_string())),
    };
  }
  Err(CanonicalLowerError::InvalidCpuResource(value.to_string()))
}

fn parse_memory_resource(raw: &str) -> Result<u64, CanonicalLowerError> {
  let value = raw.trim();
  if let Ok(bytes) = value.parse::<u64>() {
    return Ok(bytes);
  }
  if let Some((number, unit)) = value.split_once('.') {
    let parsed = number
      .trim()
      .parse::<u64>()
      .map_err(|_| CanonicalLowerError::InvalidMemoryResource(value.to_string()))?;
    let multiplier = match unit.trim() {
      "mebi" | "Mi" | "mib" => 1024_u64.pow(2),
      "gibi" | "Gi" | "gib" => 1024_u64.pow(3),
      "tebi" | "Ti" | "tib" => 1024_u64.pow(4),
      _ => {
        return Err(CanonicalLowerError::InvalidMemoryResource(
          value.to_string(),
        ));
      }
    };
    return parsed
      .checked_mul(multiplier)
      .ok_or_else(|| CanonicalLowerError::InvalidMemoryResource(value.to_string()));
  }
  Err(CanonicalLowerError::InvalidMemoryResource(
    value.to_string(),
  ))
}

fn last_segment(raw: &str) -> &str {
  raw.split('.').next_back().unwrap_or(raw).trim()
}

fn default_namespace() -> String {
  String::from("default")
}

fn normalize_runtime(raw: &str) -> Result<RuntimeKind, CanonicalLowerError> {
  match raw.trim() {
    "container" | "docker" => Ok(RuntimeKind::Docker),
    "containerd" => Ok(RuntimeKind::Containerd),
    "firecracker" => Ok(RuntimeKind::Firecracker),
    "process" => Ok(RuntimeKind::Process),
    other => Err(CanonicalLowerError::UnsupportedRuntime(other.to_string())),
  }
}

fn normalize_workload_kind(raw: &str) -> Result<WorkloadKind, CanonicalLowerError> {
  match raw.trim() {
    "service" => Ok(WorkloadKind::Service),
    "worker" => Ok(WorkloadKind::Worker),
    "job" => Ok(WorkloadKind::Job),
    "cron" => Ok(WorkloadKind::Cron),
    "stateful" => Ok(WorkloadKind::Stateful),
    other => Err(CanonicalLowerError::UnsupportedWorkloadKind(
      other.to_string(),
    )),
  }
}

fn normalize_endpoint_protocol(
  raw: Option<&str>,
  name_hint: Option<&str>,
) -> Result<EndpointProtocol, CanonicalLowerError> {
  match raw.map(str::trim).filter(|value| !value.is_empty()) {
    Some("tcp") => Ok(EndpointProtocol::Tcp),
    Some("udp") => Ok(EndpointProtocol::Udp),
    Some("http") | Some("https") => Ok(EndpointProtocol::Http),
    Some(other) => Err(CanonicalLowerError::UnsupportedEndpointProtocol(
      other.to_string(),
    )),
    None => Ok(infer_endpoint_protocol(name_hint)),
  }
}

fn infer_endpoint_protocol(name: Option<&str>) -> EndpointProtocol {
  match name.map(str::trim) {
    Some("http") | Some("https") => EndpointProtocol::Http,
    Some("udp") => EndpointProtocol::Udp,
    _ => EndpointProtocol::Tcp,
  }
}

fn interpolate_template(
  raw: &str,
  lets: &HashMap<String, String>,
) -> Result<String, CanonicalLowerError> {
  let mut out = String::new();
  let mut i = 0usize;
  while i < raw.len() {
    if raw[i..].starts_with("${") {
      let end = raw[(i + 2)..]
        .find('}')
        .map(|value| value + i + 2)
        .ok_or_else(|| CanonicalLowerError::UnclosedInterpolation(raw.to_string()))?;
      let key = raw[(i + 2)..end].trim();
      let value = lets
        .get(key)
        .ok_or_else(|| CanonicalLowerError::UnknownInterpolation(key.to_string()))?;
      out.push_str(value);
      i = end + 1;
      continue;
    }
    out.push(raw.as_bytes()[i] as char);
    i += 1;
  }
  Ok(out)
}

fn eval_replicas_expression(
  expr: &str,
  lets: &HashMap<String, String>,
) -> Result<u32, CanonicalLowerError> {
  let expr = expr.trim();
  if let Ok(value) = expr.parse::<u32>() {
    return Ok(value);
  }
  if let Some(condition) = expr.strip_prefix("if ") {
    let (cond_expr, branches) = condition
      .split_once(" then ")
      .ok_or_else(|| CanonicalLowerError::InvalidReplicas(expr.to_string()))?;
    let (then_expr, else_expr) = branches
      .split_once(" else ")
      .ok_or_else(|| CanonicalLowerError::InvalidReplicas(expr.to_string()))?;
    let (lhs, rhs) = cond_expr
      .split_once("==")
      .ok_or_else(|| CanonicalLowerError::InvalidReplicas(expr.to_string()))?;
    let key = lhs.trim();
    let expected = parse_quoted_string(rhs.trim())
      .map_err(|_| CanonicalLowerError::InvalidReplicas(expr.to_string()))?;
    let actual = lets
      .get(key)
      .ok_or_else(|| CanonicalLowerError::InvalidReplicas(expr.to_string()))?;
    let chosen = if actual == &expected {
      then_expr.trim()
    } else {
      else_expr.trim()
    };
    return chosen
      .parse::<u32>()
      .map_err(|_| CanonicalLowerError::InvalidReplicas(expr.to_string()));
  }
  Err(CanonicalLowerError::InvalidReplicas(expr.to_string()))
}

fn parse_quoted_string(raw: &str) -> Result<String, ()> {
  let value = raw.trim();
  if !(value.starts_with('"') && value.ends_with('"') && value.len() >= 2) {
    return Err(());
  }
  Ok(value[1..(value.len() - 1)].to_string())
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

fn stable_ingress_route_id(
  app: &CanonicalApplication,
  ingress: &CanonicalIngress,
  route: &CanonicalIngressRoute,
) -> String {
  let mut raw = format!(
    "{}-{}-{}-{}-{}",
    app.metadata.namespace, app.metadata.name, ingress.name, route.path, route.backend.workload
  );
  if let Some(endpoint) = route.backend.endpoint.as_deref() {
    raw.push('-');
    raw.push_str(endpoint);
  }
  format!("route-{}", sanitize_route_token(&raw))
}

fn sanitize_route_token(raw: &str) -> String {
  raw
    .chars()
    .map(|ch| {
      if ch.is_ascii_alphanumeric() {
        ch.to_ascii_lowercase()
      } else {
        '-'
      }
    })
    .collect::<String>()
    .split('-')
    .filter(|segment| !segment.is_empty())
    .collect::<Vec<_>>()
    .join("-")
}

fn ensure_route_path(path: &str) -> String {
  let trimmed = path.trim();
  if trimmed.is_empty() {
    String::from("/")
  } else if trimmed.starts_with('/') {
    trimmed.to_string()
  } else {
    format!("/{trimmed}")
  }
}

#[cfg(test)]
mod tests {
  use super::*;
  use warden_dsl_hir::lower as lower_hir;
  use warden_dsl_parser::parse;

  #[test]
  fn lowers_current_invocation_shape_into_canonical_application() {
    let raw = r#"
app("mall") {
  let env = "prod"
  let version = "1.2.3"
  volume("redisData") {}
  config("appConfig") {}
  secret("dbPassword") {}
  services {
    val redis = create("redis") {
      kind(stateful)
      runtime(container)
      image("redis:${version}")
      expose("redis") { container(6379) }
    }
    val gateway = create("gateway") {
      kind(service)
      runtime(containerd)
      image("ghcr.io/acme/gateway:${version}")
      replicas(if env == "prod" then 3 else 1)
      dependsOn(redis)
      mount(volumes.redisData, "/data")
      env {
        set("APP_CONFIG", configs.appConfig)
        set("DB_PASSWORD", secrets.dbPassword)
        set("REDIS_ADDR", services.redis.endpoint("redis"))
        set("MODE", "prod")
      }
      resources {
        cpu(500.milliCpu)
        memory(512.mebi)
      }
      health {
        readiness { http("/ready", endpoint("http")) }
        liveness { http("/live", services.gateway.endpoint("http")) }
      }
    }
  }
  ingress("public") {
    host("mall.example.com")
    route("/") { backend(services.gateway) port("http") }
  }
}
"#;
    let ast = parse(raw).unwrap();
    let hir = lower_hir(&ast).unwrap();
    let canonical = lower(&hir).unwrap();

    assert_eq!(canonical.metadata.name, "mall");
    assert_eq!(canonical.metadata.namespace, "default");
    assert_eq!(canonical.configs[0].name, "appConfig");
    assert_eq!(canonical.secrets[0].name, "dbPassword");
    assert_eq!(canonical.volumes[0].name, "redisData");
    assert_eq!(canonical.workloads.len(), 2);
    assert_eq!(canonical.workloads[0].name, "redis");
    assert_eq!(canonical.workloads[0].kind, WorkloadKind::Stateful);
    assert_eq!(canonical.workloads[0].runtime, RuntimeKind::Docker);
    assert_eq!(
      canonical.workloads[0].run.image.as_deref(),
      Some("redis:1.2.3")
    );
    assert_eq!(canonical.workloads[0].endpoints[0].name, "redis");
    assert_eq!(canonical.workloads[0].endpoints[0].port, 6379);
    assert_eq!(canonical.workloads[1].runtime, RuntimeKind::Containerd);
    assert_eq!(canonical.workloads[1].replicas, Some(3));
    assert_eq!(canonical.workloads[1].depends_on[0].name, "redis");
    assert_eq!(canonical.workloads[1].mounts[0].volume.name, "redisData");
    assert_eq!(canonical.workloads[1].mounts[0].target, "/data");
    assert_eq!(canonical.workloads[1].run.env.len(), 4);
    assert!(matches!(
      canonical.workloads[1].run.env[0].value,
      CanonicalEnvValue::ConfigRef(_)
    ));
    assert!(matches!(
      canonical.workloads[1].run.env[1].value,
      CanonicalEnvValue::SecretRef(_)
    ));
    assert!(matches!(
      canonical.workloads[1].run.env[2].value,
      CanonicalEnvValue::EndpointRef(_)
    ));
    assert!(matches!(
      canonical.workloads[1].run.env[3].value,
      CanonicalEnvValue::String(_)
    ));
    assert_eq!(
      canonical.workloads[1]
        .run
        .resources
        .as_ref()
        .and_then(|value| value.cpu_millis),
      Some(500)
    );
    assert_eq!(
      canonical.workloads[1]
        .run
        .resources
        .as_ref()
        .and_then(|value| value.memory_bytes),
      Some(512 * 1024 * 1024)
    );
    assert_eq!(
      canonical.workloads[1]
        .health
        .as_ref()
        .and_then(|value| value.readiness.as_ref())
        .map(|value| value.path.as_str()),
      Some("/ready")
    );
    assert_eq!(
      canonical.workloads[1]
        .health
        .as_ref()
        .and_then(|value| value.liveness.as_ref())
        .map(|value| value.endpoint.workload.as_str()),
      Some("gateway")
    );
    assert_eq!(canonical.ingresses[0].host, "mall.example.com");
    assert_eq!(canonical.ingresses[0].routes[0].backend.workload, "gateway");
    assert_eq!(
      canonical.ingresses[0].routes[0].backend.endpoint.as_deref(),
      Some("http")
    );
  }

  #[test]
  fn lowers_top_level_workload_blocks_into_canonical_application() {
    let raw = r#"
app("mall") {
  let env = "prod"
  config("appConfig") {}
  secret("dbPassword") {}
  volume("redisData") {}
  workload("redis") {
    kind(stateful)
    runtime(containerd)
    image("redis:7")
    endpoint("redis") { port(6379) protocol(tcp) }
  }
  workload("gateway") {
    kind(worker)
    runtime(container)
    image("ghcr.io/acme/gateway:${env}")
    replicas(if env == "prod" then 3 else 1)
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
    let canonical = lower(&hir).unwrap();

    assert_eq!(canonical.configs[0].name, "appConfig");
    assert_eq!(canonical.secrets[0].name, "dbPassword");
    assert_eq!(canonical.volumes[0].name, "redisData");
    assert_eq!(canonical.workloads.len(), 2);
    assert_eq!(canonical.workloads[0].name, "redis");
    assert_eq!(canonical.workloads[0].kind, WorkloadKind::Stateful);
    assert_eq!(canonical.workloads[0].runtime, RuntimeKind::Containerd);
    assert_eq!(canonical.workloads[0].endpoints[0].port, 6379);
    assert_eq!(
      canonical.workloads[0].endpoints[0].protocol,
      EndpointProtocol::Tcp
    );
    assert_eq!(canonical.workloads[1].name, "gateway");
    assert_eq!(canonical.workloads[1].kind, WorkloadKind::Worker);
    assert_eq!(
      canonical.workloads[1].run.image.as_deref(),
      Some("ghcr.io/acme/gateway:prod")
    );
    assert_eq!(canonical.workloads[1].replicas, Some(3));
    assert_eq!(canonical.workloads[1].depends_on[0].name, "redis");
    assert_eq!(canonical.workloads[1].mounts[0].volume.name, "redisData");
    assert_eq!(canonical.workloads[1].run.env.len(), 4);
    assert_eq!(
      canonical.workloads[1]
        .run
        .resources
        .as_ref()
        .and_then(|value| value.cpu_millis),
      Some(500)
    );
    assert_eq!(
      canonical.workloads[1]
        .health
        .as_ref()
        .and_then(|value| value.readiness.as_ref())
        .map(|value| value.path.as_str()),
      Some("/ready")
    );
    assert_eq!(canonical.workloads[1].endpoints[0].name, "http");
    assert_eq!(
      canonical.workloads[1].endpoints[0].protocol,
      EndpointProtocol::Http
    );
    assert_eq!(canonical.ingresses[0].routes[0].backend.workload, "gateway");
  }

  #[test]
  fn compiles_canonical_apply_output_with_explicit_ingress_route_specs() {
    let raw = r#"
app("mall") {
  workload("gateway") {
    runtime(containerd)
    endpoint("http") { port(8080) protocol(http) }
  }
  ingress("public") {
    host("mall.example.com")
    route("/") { backend(workloads.gateway.endpoint("http")) }
  }
}
"#;
    let ast = parse(raw).unwrap();
    let hir = lower_hir(&ast).unwrap();
    let canonical = lower(&hir).unwrap();
    let apply = compile_apply_output(&canonical);

    assert_eq!(apply.application, "mall");
    assert_eq!(apply.namespace, "default");
    assert_eq!(apply.ingress_routes.len(), 1);
    assert_eq!(apply.ingress_routes[0].ingress_name, "public");
    assert_eq!(apply.ingress_routes[0].host, "mall.example.com");
    assert_eq!(apply.ingress_routes[0].path_prefix, "/");
    assert_eq!(apply.ingress_routes[0].protocol, "http");
    assert_eq!(apply.ingress_routes[0].listen_port, 8088);
    assert_eq!(apply.ingress_routes[0].backend.workload, "gateway");
    assert_eq!(
      apply.ingress_routes[0].backend.endpoint.as_deref(),
      Some("http")
    );
    assert!(
      apply.ingress_routes[0]
        .id
        .starts_with("route-default-mall-public")
    );
  }
}
