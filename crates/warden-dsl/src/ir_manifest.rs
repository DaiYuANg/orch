//! Map [`warden_dsl_ir::IrApplication`] (and optionally [`warden_dsl_hir::HirDocument`]) into
//! [`crate::model::ApplicationManifest`] for validation and [`crate::compile_manifest`].

use crate::model::{
  ApplicationManifest, IngressSpec, Metadata, ServiceSpec, Spec, WorkloadSpec,
  default_namespace_string, default_runtime_string, eval_replicas_expression,
  interpolate_template, normalize_runtime_symbol,
};
use anyhow::Context;
use std::collections::HashMap;
use warden_dsl_ast::ExprAst;
use warden_dsl_hir::HirDocument;
use warden_dsl_ir::IrApplication;

const API_VERSION: &str = "warden.io/v1alpha1";
const KIND: &str = "Application";

/// String `let` bindings from HIR (only string literals are kept; required for `replicas(if ...)`).
pub fn hir_string_lets(hir: &HirDocument) -> anyhow::Result<HashMap<String, String>> {
  let mut m = HashMap::new();
  for l in &hir.lets {
    match &l.expr {
      ExprAst::String(s) => {
        m.insert(l.name.to_string(), s.clone());
      }
      _ => {
        anyhow::bail!(
          "let {} must use a string literal for manifest mapping",
          l.name
        );
      }
    }
  }
  Ok(m)
}

/// Build ingress route bindings keyed by service **binding** (alias), matching the legacy string parser.
fn ingress_bindings_from_ir(ir: &IrApplication) -> HashMap<String, (String, String)> {
  let mut bindings = HashMap::new();
  for ing in &ir.ingress {
    let host = ing
      .host
      .clone()
      .unwrap_or_else(|| "warden.local".to_string());
    for route in &ing.routes {
      let Some(path) = route.path.as_ref() else {
        continue;
      };
      let Some(backend) = route.backend.as_ref() else {
        continue;
      };
      let alias = backend
        .split('.')
        .next_back()
        .unwrap_or("")
        .trim()
        .to_string();
      if alias.is_empty() || bindings.contains_key(&alias) {
        continue;
      }
      bindings.insert(alias, (host.clone(), path.clone()));
    }
  }
  bindings
}

/// Convert IR + `let` map into an [`ApplicationManifest`] (same shape as YAML / legacy invocation parse).
pub fn application_manifest_from_ir(
  ir: &IrApplication,
  lets: &HashMap<String, String>,
) -> anyhow::Result<ApplicationManifest> {
  let ingress_bindings = ingress_bindings_from_ir(ir);
  let alias_to_name: HashMap<String, String> = ir
    .workloads
    .iter()
    .map(|w| (w.binding.clone(), w.name.clone()))
    .collect();

  let workloads = ir
    .workloads
    .iter()
    .map(|w| {
      let runtime = w
        .runtime
        .as_deref()
        .map(normalize_runtime_symbol)
        .filter(|s| !s.is_empty())
        .unwrap_or_else(default_runtime_string);
      let replicas = w
        .replicas
        .as_deref()
        .map(|expr| eval_replicas_expression(expr, lets))
        .transpose()?
        .flatten();
      let depends_on = w
        .depends_on
        .iter()
        .map(|dep| {
          dep
            .split('.')
            .next_back()
            .and_then(|token| alias_to_name.get(token.trim()))
            .cloned()
            .unwrap_or_else(|| dep.clone())
        })
        .collect::<Vec<_>>();
      let ingress = ingress_bindings.get(&w.binding).map(|(host, path)| IngressSpec {
        enabled: Some(true),
        host: Some(host.clone()),
        path: Some(path.clone()),
        listen_port: None,
      });
      let image = match w.image.as_deref() {
        Some(img) => Some(interpolate_template(img, lets)?),
        None => None,
      };
      Ok::<_, anyhow::Error>(WorkloadSpec {
        name: w.name.clone(),
        runtime,
        replicas,
        depends_on,
        image,
        process: None,
        firecracker: None,
        service: w.service_port.map(|port| ServiceSpec {
          port: Some(port),
          backend: None,
        }),
        ingress,
        dns: None,
        scheduling: None,
      })
    })
    .collect::<Result<Vec<_>, _>>()?;

  Ok(ApplicationManifest {
    api_version: API_VERSION.to_string(),
    kind: KIND.to_string(),
    metadata: Metadata {
      name: ir.app_name.clone(),
      namespace: default_namespace_string(),
    },
    spec: Spec { workloads },
  })
}

/// `HIR → IR → ApplicationManifest` with `let` extraction from HIR.
pub fn application_manifest_from_hir(hir: &HirDocument) -> anyhow::Result<ApplicationManifest> {
  let lets = hir_string_lets(hir).context("hir let bindings")?;
  let ir = warden_dsl_ir::lower(hir);
  application_manifest_from_ir(&ir, &lets)
}
