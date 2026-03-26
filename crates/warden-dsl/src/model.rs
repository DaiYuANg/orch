use anyhow::{Context, bail};
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};
use std::path::Path;

const API_VERSION: &str = "warden.io/v1alpha1";
const KIND: &str = "Application";
const SUPPORTED_RUNTIMES: [&str; 4] = ["docker", "containerd", "process", "firecracker"];

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ApplicationManifest {
  pub api_version: String,
  pub kind: String,
  pub metadata: Metadata,
  pub spec: Spec,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Metadata {
  pub name: String,
  #[serde(default = "default_namespace")]
  pub namespace: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct Spec {
  #[serde(default)]
  pub workloads: Vec<WorkloadSpec>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkloadSpec {
  pub name: String,
  #[serde(default = "default_runtime")]
  pub runtime: String,
  pub replicas: Option<u32>,
  #[serde(default)]
  pub depends_on: Vec<String>,
  pub image: Option<String>,
  pub process: Option<ProcessSpec>,
  pub firecracker: Option<FirecrackerSpec>,
  pub service: Option<ServiceSpec>,
  pub ingress: Option<IngressSpec>,
  pub dns: Option<DnsSpec>,
  pub scheduling: Option<SchedulingSpec>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProcessSpec {
  pub command: Option<String>,
  #[serde(default)]
  pub args: Vec<String>,
  #[serde(default)]
  pub env: std::collections::BTreeMap<String, String>,
  pub cwd: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct FirecrackerSpec {
  pub config: Option<String>,
  pub kernel_image: Option<String>,
  pub rootfs: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServiceSpec {
  pub port: Option<u16>,
  pub backend: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct IngressSpec {
  pub enabled: Option<bool>,
  pub host: Option<String>,
  pub path: Option<String>,
  pub listen_port: Option<u16>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DnsSpec {
  pub enabled: Option<bool>,
  pub ttl: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SchedulingSpec {
  pub stateful: Option<bool>,
  pub allow_leader: Option<bool>,
  #[serde(default)]
  pub preferred_nodes: Vec<String>,
}

impl ApplicationManifest {
  pub fn validate(&self) -> anyhow::Result<()> {
    if self.api_version.trim() != API_VERSION {
      bail!("apiVersion must be {}", API_VERSION);
    }
    if self.kind.trim() != KIND {
      bail!("kind must be {}", KIND);
    }
    let app = self.metadata.name.trim();
    if app.is_empty() {
      bail!("metadata.name is required");
    }
    if !is_name_token(app) {
      bail!(
        "metadata.name contains invalid characters: {}",
        self.metadata.name
      );
    }
    let namespace = self.namespace();
    if namespace.is_empty() {
      bail!("metadata.namespace is required");
    }
    if !is_name_token(namespace) {
      bail!(
        "metadata.namespace contains invalid characters: {}",
        self.metadata.namespace
      );
    }
    if self.spec.workloads.is_empty() {
      bail!("spec.workloads must not be empty");
    }

    let mut names = HashSet::new();
    for workload in &self.spec.workloads {
      let name = workload.name.trim();
      if !is_name_token(name) {
        bail!("invalid workload name: {}", workload.name);
      }
      if !names.insert(name.to_string()) {
        bail!("duplicate workload name: {}", workload.name);
      }
      let runtime = workload.runtime.trim();
      if runtime.is_empty() {
        bail!("runtime is required for workload {}", workload.name);
      }
      if !SUPPORTED_RUNTIMES.contains(&runtime) {
        bail!(
          "unsupported runtime '{}' for workload {}; supported: {}",
          workload.runtime,
          workload.name,
          SUPPORTED_RUNTIMES.join(", ")
        );
      }
      for dep in &workload.depends_on {
        if !is_name_token(dep.trim()) {
          bail!(
            "dependsOn contains invalid workload reference '{}' for workload {}",
            dep,
            workload.name
          );
        }
      }
      if let Some(service) = workload.service.as_ref()
        && matches!(service.port, Some(0))
      {
        bail!(
          "service.port must be between 1 and 65535 for workload {}",
          workload.name
        );
      }
      if let Some(ingress) = workload.ingress.as_ref()
        && matches!(ingress.listen_port, Some(0))
      {
        bail!(
          "ingress.listenPort must be between 1 and 65535 for workload {}",
          workload.name
        );
      }
      if runtime == "process" {
        let process = workload
          .process
          .as_ref()
          .with_context(|| format!("process block required for workload {}", workload.name))?;
        let has_command = non_empty(process.command.as_deref()).is_some();
        if !has_command {
          bail!("process.command is required for workload {}", workload.name);
        }
      }
      if let Some(ingress) = workload.ingress.as_ref()
        && let Some(path) = ingress.path.as_deref()
        && !path.trim().starts_with('/')
      {
        bail!(
          "ingress.path must start with / for workload {}",
          workload.name
        );
      }
      if runtime == "firecracker" {
        let firecracker = workload
          .firecracker
          .as_ref()
          .with_context(|| format!("firecracker block required for workload {}", workload.name))?;
        let has_config = non_empty(firecracker.config.as_deref()).is_some();
        let has_kernel = non_empty(firecracker.kernel_image.as_deref()).is_some();
        let has_rootfs = non_empty(firecracker.rootfs.as_deref()).is_some();
        if !(has_config || has_kernel && has_rootfs) {
          bail!(
            "firecracker workload {} needs firecracker.config or kernelImage+rootfs",
            workload.name
          );
        }
      }
    }
    for workload in &self.spec.workloads {
      for dep in &workload.depends_on {
        let dep = dep.trim();
        if dep == workload.name.trim() {
          bail!("workload {} cannot depend on itself", workload.name);
        }
        if !names.contains(dep) {
          bail!(
            "workload {} depends on unknown workload {}",
            workload.name,
            dep
          );
        }
      }
    }
    Ok(())
  }

  pub fn namespace(&self) -> &str {
    self.metadata.namespace.trim()
  }

  pub fn application(&self) -> &str {
    self.metadata.name.trim()
  }
}

pub fn load_manifest(path: &Path) -> anyhow::Result<ApplicationManifest> {
  let raw = std::fs::read_to_string(path)
    .with_context(|| format!("read manifest file {}", path.display()))?;
  parse_manifest_source(&raw).with_context(|| format!("decode manifest source {}", path.display()))
}

pub fn parse_manifest_source(raw: &str) -> anyhow::Result<ApplicationManifest> {
  if let Ok(manifest) = serde_yaml::from_str::<ApplicationManifest>(raw) {
    manifest.validate()?;
    return Ok(manifest);
  }
  let manifest = parse_manifest_invocation(raw)?;
  manifest.validate()?;
  Ok(manifest)
}

pub fn parse_manifest_yaml(raw: &str) -> anyhow::Result<ApplicationManifest> {
  let manifest: ApplicationManifest = serde_yaml::from_str(raw).context("decode manifest yaml")?;
  manifest.validate()?;
  Ok(manifest)
}

fn parse_manifest_invocation(raw: &str) -> anyhow::Result<ApplicationManifest> {
  let source = raw.trim();
  let app_call_pos = source
    .find("app(")
    .ok_or_else(|| anyhow::anyhow!("expected app(\"name\") at top level"))?;
  let app_open_paren = app_call_pos + "app".len();
  let app_close_paren = find_matching(source, app_open_paren, '(', ')')
    .ok_or_else(|| anyhow::anyhow!("unclosed app(...) declaration"))?;
  let app_name = parse_quoted_string(source[(app_open_paren + 1)..app_close_paren].trim())
    .context("parse app name")?;
  let app_open_brace = find_next_non_ws(source, app_close_paren + 1)
    .ok_or_else(|| anyhow::anyhow!("missing app body block"))?;
  if source.as_bytes()[app_open_brace] as char != '{' {
    bail!("expected '{{' after app(...)");
  }
  let app_close_brace = find_matching(source, app_open_brace, '{', '}')
    .ok_or_else(|| anyhow::anyhow!("unclosed app body"))?;
  let app_body = &source[(app_open_brace + 1)..app_close_brace];

  let lets = parse_let_bindings(app_body)?;
  let ingress_bindings = parse_ingress_bindings(app_body)?;
  let workloads = if let Some(services_body) = extract_named_block(app_body, "services")? {
    parse_services_block(services_body, &lets, &ingress_bindings)?
  } else {
    Vec::new()
  };

  Ok(ApplicationManifest {
    api_version: API_VERSION.to_string(),
    kind: KIND.to_string(),
    metadata: Metadata {
      name: app_name,
      namespace: default_namespace(),
    },
    spec: Spec { workloads },
  })
}

fn parse_let_bindings(app_body: &str) -> anyhow::Result<HashMap<String, String>> {
  let mut lets = HashMap::new();
  for line in app_body.lines() {
    let trimmed = line.trim();
    if !trimmed.starts_with("let ") {
      continue;
    }
    let rest = trimmed["let ".len()..].trim();
    let (name, value_expr) = rest
      .split_once('=')
      .ok_or_else(|| anyhow::anyhow!("invalid let binding: {}", trimmed))?;
    let name = name.trim();
    if !is_identifier(name) {
      bail!("invalid let variable name: {}", name);
    }
    let value = parse_quoted_string(value_expr.trim())?;
    lets.insert(name.to_string(), value);
  }
  Ok(lets)
}

fn parse_services_block(
  services_body: &str,
  lets: &HashMap<String, String>,
  ingress_bindings: &HashMap<String, (String, String)>,
) -> anyhow::Result<Vec<WorkloadSpec>> {
  #[derive(Debug)]
  struct Draft {
    alias: String,
    name: String,
    runtime: String,
    image: Option<String>,
    replicas: Option<u32>,
    depends_on: Vec<String>,
    service_port: Option<u16>,
  }

  let mut drafts = Vec::new();
  let mut cursor = 0usize;
  while let Some(val_pos) = find_keyword_from(services_body, "val", cursor) {
    let mut i = val_pos + 3;
    i = skip_ws(services_body, i);
    let (alias, next) = parse_identifier_at(services_body, i)
      .ok_or_else(|| anyhow::anyhow!("expected variable name after val"))?;
    i = skip_ws(services_body, next);
    i = expect_char(services_body, i, '=')?;
    i = skip_ws(services_body, i);
    i = expect_word(services_body, i, "create")?;
    i = skip_ws(services_body, i);
    i = expect_char(services_body, i, '(')?;
    let close_paren = find_matching(services_body, i - 1, '(', ')')
      .ok_or_else(|| anyhow::anyhow!("unclosed create(...)"))?;
    let workload_name = parse_quoted_string(services_body[i..close_paren].trim())?;
    i = skip_ws(services_body, close_paren + 1);
    let open_brace = expect_char(services_body, i, '{')? - 1;
    let close_brace = find_matching(services_body, open_brace, '{', '}')
      .ok_or_else(|| anyhow::anyhow!("unclosed create {{...}} block"))?;
    let body = &services_body[(open_brace + 1)..close_brace];
    cursor = close_brace + 1;

    let runtime = parse_symbol_invocation(body, "runtime")
      .map(|value| normalize_runtime_symbol(&value))
      .unwrap_or_else(default_runtime);
    let image = parse_string_invocation(body, "image")
      .map(|v| interpolate_template(&v, lets))
      .transpose()?;
    let replicas = parse_replicas_invocation(body, lets)?;
    let depends_on = parse_depends_on_invocation(body)?;
    let service_port = parse_expose_container_port(body)?;

    drafts.push(Draft {
      alias,
      name: workload_name,
      runtime,
      image,
      replicas,
      depends_on,
      service_port,
    });
  }

  let alias_to_name = drafts
    .iter()
    .map(|draft| (draft.alias.clone(), draft.name.clone()))
    .collect::<HashMap<_, _>>();

  let workloads = drafts
    .into_iter()
    .map(|draft| {
      let depends_on = draft
        .depends_on
        .into_iter()
        .map(|dep| {
          dep
            .split('.')
            .next_back()
            .and_then(|token| alias_to_name.get(token).cloned())
            .unwrap_or(dep)
        })
        .collect::<Vec<_>>();
      WorkloadSpec {
        name: draft.name,
        runtime: draft.runtime,
        replicas: draft.replicas,
        depends_on,
        image: draft.image,
        process: None,
        firecracker: None,
        service: draft.service_port.map(|port| ServiceSpec {
          port: Some(port),
          backend: None,
        }),
        ingress: ingress_bindings
          .get(&draft.alias)
          .map(|(host, path)| IngressSpec {
            enabled: Some(true),
            host: Some(host.clone()),
            path: Some(path.clone()),
            listen_port: None,
          }),
        dns: None,
        scheduling: None,
      }
    })
    .collect::<Vec<_>>();
  Ok(workloads)
}

fn extract_named_block<'a>(raw: &'a str, name: &str) -> anyhow::Result<Option<&'a str>> {
  if let Some(pos) = find_keyword_from(raw, name, 0) {
    let mut open = find_next_non_ws(raw, pos + name.len())
      .ok_or_else(|| anyhow::anyhow!("missing '{{' after {}", name))?;
    if raw.as_bytes()[open] as char == '(' {
      let close_paren = find_matching(raw, open, '(', ')')
        .ok_or_else(|| anyhow::anyhow!("unclosed {}(...)", name))?;
      open = find_next_non_ws(raw, close_paren + 1)
        .ok_or_else(|| anyhow::anyhow!("missing '{{' after {}(...)", name))?;
    }
    if raw.as_bytes()[open] as char != '{' {
      bail!("expected '{{' after {}", name);
    }
    let close = find_matching(raw, open, '{', '}')
      .ok_or_else(|| anyhow::anyhow!("unclosed {} block", name))?;
    return Ok(Some(&raw[(open + 1)..close]));
  }
  Ok(None)
}

fn parse_symbol_invocation(raw: &str, name: &str) -> Option<String> {
  let arg = find_invocation_arg(raw, name)?;
  let symbol = arg.trim();
  if is_identifier(symbol) {
    Some(symbol.to_string())
  } else {
    None
  }
}

fn parse_string_invocation(raw: &str, name: &str) -> Option<String> {
  let arg = find_invocation_arg(raw, name)?;
  parse_quoted_string(arg.trim()).ok()
}

fn parse_replicas_invocation(
  raw: &str,
  lets: &HashMap<String, String>,
) -> anyhow::Result<Option<u32>> {
  let Some(arg) = find_invocation_arg(raw, "replicas") else {
    return Ok(None);
  };
  eval_replicas_expression(arg.trim(), lets)
}

/// Evaluates a `replicas(...)` argument (plain integer or `if a == "b" then n else m`).
pub fn eval_replicas_expression(
  expr: &str,
  lets: &HashMap<String, String>,
) -> anyhow::Result<Option<u32>> {
  let expr = expr.trim();
  if expr.is_empty() {
    return Ok(None);
  }
  if let Ok(value) = expr.parse::<u32>() {
    return Ok(Some(value));
  }
  if let Some(condition) = expr.strip_prefix("if ") {
    let (cond_expr, branches) = condition
      .split_once(" then ")
      .ok_or_else(|| anyhow::anyhow!("invalid if expression in replicas(...)"))?;
    let (then_expr, else_expr) = branches
      .split_once(" else ")
      .ok_or_else(|| anyhow::anyhow!("invalid if expression in replicas(...)"))?;
    let (lhs, rhs) = cond_expr
      .split_once("==")
      .ok_or_else(|| anyhow::anyhow!("replicas if-condition must use =="))?;
    let key = lhs.trim();
    let expected = parse_quoted_string(rhs.trim())?;
    let actual = lets
      .get(key)
      .ok_or_else(|| anyhow::anyhow!("unknown let variable '{}' in replicas(...)", key))?;
    let chosen = if actual == &expected {
      then_expr.trim()
    } else {
      else_expr.trim()
    };
    let value = chosen
      .parse::<u32>()
      .with_context(|| format!("invalid replicas value '{}'", chosen))?;
    return Ok(Some(value));
  }
  bail!("unsupported replicas expression: {}", expr)
}

fn parse_depends_on_invocation(raw: &str) -> anyhow::Result<Vec<String>> {
  let Some(arg) = find_invocation_arg(raw, "dependsOn") else {
    return Ok(Vec::new());
  };
  let deps = arg
    .split(',')
    .map(str::trim)
    .filter(|item| !item.is_empty())
    .map(|item| item.to_string())
    .collect::<Vec<_>>();
  Ok(deps)
}

fn parse_expose_container_port(raw: &str) -> anyhow::Result<Option<u16>> {
  let Some(expose_body) = extract_named_block(raw, "expose")? else {
    return Ok(None);
  };
  let Some(arg) = find_invocation_arg(expose_body, "container") else {
    return Ok(None);
  };
  let port = arg
    .trim()
    .parse::<u16>()
    .with_context(|| format!("invalid container port '{}'", arg.trim()))?;
  Ok(Some(port))
}

fn parse_ingress_bindings(app_body: &str) -> anyhow::Result<HashMap<String, (String, String)>> {
  let mut bindings = HashMap::new();
  let mut cursor = 0usize;
  while let Some(pos) = find_keyword_from(app_body, "ingress", cursor) {
    let mut i = pos + "ingress".len();
    i = skip_ws(app_body, i);
    if app_body.as_bytes().get(i).copied().map(char::from) != Some('(') {
      cursor = pos + 1;
      continue;
    }
    let close_paren = find_matching(app_body, i, '(', ')')
      .ok_or_else(|| anyhow::anyhow!("unclosed ingress(...)"))?;
    i = skip_ws(app_body, close_paren + 1);
    let open_brace = expect_char(app_body, i, '{')? - 1;
    let close_brace = find_matching(app_body, open_brace, '{', '}')
      .ok_or_else(|| anyhow::anyhow!("unclosed ingress block"))?;
    let ingress_body = &app_body[(open_brace + 1)..close_brace];
    cursor = close_brace + 1;

    let host =
      parse_string_invocation(ingress_body, "host").unwrap_or_else(|| "warden.local".to_string());

    let mut route_cursor = 0usize;
    while let Some(route_pos) = find_keyword_from(ingress_body, "route", route_cursor) {
      let mut j = route_pos + "route".len();
      j = skip_ws(ingress_body, j);
      if ingress_body.as_bytes().get(j).copied().map(char::from) != Some('(') {
        route_cursor = route_pos + 1;
        continue;
      }
      let route_close_paren = find_matching(ingress_body, j, '(', ')')
        .ok_or_else(|| anyhow::anyhow!("unclosed route(...)"))?;
      let path = parse_quoted_string(ingress_body[(j + 1)..route_close_paren].trim())?;
      j = skip_ws(ingress_body, route_close_paren + 1);
      let route_open_brace = expect_char(ingress_body, j, '{')? - 1;
      let route_close_brace = find_matching(ingress_body, route_open_brace, '{', '}')
        .ok_or_else(|| anyhow::anyhow!("unclosed route block"))?;
      let route_body = &ingress_body[(route_open_brace + 1)..route_close_brace];
      route_cursor = route_close_brace + 1;

      if let Some(backend_arg) = find_invocation_arg(route_body, "backend") {
        let backend_alias = backend_arg
          .trim()
          .split('.')
          .next_back()
          .unwrap_or("")
          .trim()
          .to_string();
        if !backend_alias.is_empty() && !bindings.contains_key(&backend_alias) {
          bindings.insert(backend_alias, (host.clone(), path.clone()));
        }
      }
    }
  }
  Ok(bindings)
}

fn find_invocation_arg<'a>(raw: &'a str, name: &str) -> Option<&'a str> {
  let mut cursor = 0usize;
  while let Some(pos) = find_keyword_from(raw, name, cursor) {
    let mut i = pos + name.len();
    i = skip_ws(raw, i);
    if raw.as_bytes().get(i).copied().map(char::from) != Some('(') {
      cursor = pos + 1;
      continue;
    }
    let close = find_matching(raw, i, '(', ')')?;
    return Some(&raw[(i + 1)..close]);
  }
  None
}

pub(crate) fn interpolate_template(
  raw: &str,
  lets: &HashMap<String, String>,
) -> anyhow::Result<String> {
  let mut out = String::new();
  let mut i = 0usize;
  while i < raw.len() {
    if raw[i..].starts_with("${") {
      let end = raw[(i + 2)..]
        .find('}')
        .map(|value| value + i + 2)
        .ok_or_else(|| anyhow::anyhow!("unclosed interpolation in '{}'", raw))?;
      let key = raw[(i + 2)..end].trim();
      let value = lets
        .get(key)
        .ok_or_else(|| anyhow::anyhow!("unknown let variable '{}' in interpolation", key))?;
      out.push_str(value);
      i = end + 1;
      continue;
    }
    let ch = raw.as_bytes()[i] as char;
    out.push(ch);
    i += 1;
  }
  Ok(out)
}

fn parse_quoted_string(raw: &str) -> anyhow::Result<String> {
  let value = raw.trim();
  if !(value.starts_with('"') && value.ends_with('"') && value.len() >= 2) {
    bail!("expected quoted string, got '{}'", value);
  }
  Ok(value[1..(value.len() - 1)].to_string())
}

fn find_matching(raw: &str, open_idx: usize, open: char, close: char) -> Option<usize> {
  let mut depth = 0usize;
  let mut in_string = false;
  let bytes = raw.as_bytes();
  let mut i = open_idx;
  while i < bytes.len() {
    let ch = bytes[i] as char;
    if ch == '"' && (i == 0 || bytes[i - 1] as char != '\\') {
      in_string = !in_string;
      i += 1;
      continue;
    }
    if in_string {
      i += 1;
      continue;
    }
    if ch == open {
      depth += 1;
    } else if ch == close {
      depth = depth.saturating_sub(1);
      if depth == 0 {
        return Some(i);
      }
    }
    i += 1;
  }
  None
}

fn find_next_non_ws(raw: &str, from: usize) -> Option<usize> {
  raw[from..]
    .char_indices()
    .find(|(_, ch)| !ch.is_whitespace())
    .map(|(idx, _)| idx + from)
}

fn find_keyword_from(raw: &str, keyword: &str, from: usize) -> Option<usize> {
  let mut cursor = from;
  while cursor < raw.len() {
    let rel = raw[cursor..].find(keyword)?;
    let pos = cursor + rel;
    let prev_ok = pos == 0 || !is_identifier_char(raw.as_bytes()[pos - 1] as char);
    let next_idx = pos + keyword.len();
    let next_ok = next_idx >= raw.len() || !is_identifier_char(raw.as_bytes()[next_idx] as char);
    if prev_ok && next_ok {
      return Some(pos);
    }
    cursor = pos + 1;
  }
  None
}

fn skip_ws(raw: &str, from: usize) -> usize {
  match find_next_non_ws(raw, from) {
    Some(idx) => idx,
    None => raw.len(),
  }
}

fn expect_char(raw: &str, at: usize, expected: char) -> anyhow::Result<usize> {
  let idx = skip_ws(raw, at);
  let found = raw
    .as_bytes()
    .get(idx)
    .copied()
    .map(char::from)
    .ok_or_else(|| anyhow::anyhow!("expected '{}', got end-of-input", expected))?;
  if found != expected {
    bail!("expected '{}', got '{}'", expected, found);
  }
  Ok(idx + 1)
}

fn expect_word(raw: &str, at: usize, word: &str) -> anyhow::Result<usize> {
  let idx = skip_ws(raw, at);
  if raw[idx..].starts_with(word) {
    Ok(idx + word.len())
  } else {
    bail!("expected '{}'", word)
  }
}

fn parse_identifier_at(raw: &str, at: usize) -> Option<(String, usize)> {
  let idx = skip_ws(raw, at);
  let mut end = idx;
  for ch in raw[idx..].chars() {
    if is_identifier_char(ch) {
      end += ch.len_utf8();
    } else {
      break;
    }
  }
  if end == idx {
    return None;
  }
  Some((raw[idx..end].to_string(), end))
}

fn is_identifier(raw: &str) -> bool {
  let mut chars = raw.chars();
  match chars.next() {
    Some(ch) if ch.is_ascii_alphabetic() || ch == '_' => {}
    _ => return false,
  }
  chars.all(is_identifier_char)
}

fn is_identifier_char(ch: char) -> bool {
  ch.is_ascii_alphanumeric() || ch == '_'
}

pub(crate) fn normalize_runtime_symbol(value: &str) -> String {
  match value {
    "container" => "docker".to_string(),
    other => other.to_string(),
  }
}

pub(crate) fn default_namespace_string() -> String {
  String::from("default")
}

fn default_namespace() -> String {
  default_namespace_string()
}

pub(crate) fn default_runtime_string() -> String {
  String::from("docker")
}

fn default_runtime() -> String {
  default_runtime_string()
}

fn is_name_token(value: &str) -> bool {
  !value.is_empty()
    && value
      .chars()
      .all(|ch| ch.is_ascii_alphanumeric() || ch == '-' || ch == '_' || ch == '.')
}

pub(crate) fn non_empty(value: Option<&str>) -> Option<String> {
  value
    .map(str::trim)
    .filter(|v| !v.is_empty())
    .map(ToString::to_string)
}
