use crate::cli_args::{DslApplyArgs, DslDeleteArgs, DslFileArgs, DslPlanArgs};
use serde::{Deserialize, Serialize};
use std::path::Path;
use warden_client::WardenClient;
use warden_dsl::{CompiledManifest, ManifestPlan, build_plan, compile_manifest, load_manifest};
use warden_types::WorkloadSummary;
use warden_types::dsl::{DslApplyRequest, DslApplyResult};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DslDeleteResult {
  pub application: String,
  pub namespace: String,
  pub stopped: Vec<String>,
  pub warnings: Vec<String>,
}

pub async fn run_plan(client: &WardenClient, args: &DslPlanArgs) -> anyhow::Result<ManifestPlan> {
  let compiled = load_and_compile(&args.file)?;
  if args.strict && !compiled.warnings.is_empty() {
    return Err(anyhow::anyhow!(
      "dsl strict mode rejected {} warnings",
      compiled.warnings.len()
    ));
  }
  let existing = fetch_workloads(client).await?;
  let names = existing
    .into_iter()
    .map(|item| item.name)
    .collect::<Vec<_>>();
  Ok(build_plan(&compiled, &names))
}

pub fn run_render(args: &DslFileArgs) -> anyhow::Result<CompiledManifest> {
  load_and_compile(&args.file)
}

pub async fn run_apply(
  client: &WardenClient,
  args: &DslApplyArgs,
) -> anyhow::Result<DslApplyResult> {
  let yaml = std::fs::read_to_string(&args.file)
    .map_err(|err| anyhow::anyhow!("read manifest {} failed: {}", args.file, err))?;
  let req = DslApplyRequest {
    manifest_yaml: yaml,
    prune: args.prune,
    strict: args.strict,
    concurrency: args.concurrency.max(1).min(32),
  };
  client.post("/dsl/apply", &req).await
}

pub async fn run_delete(
  client: &WardenClient,
  args: &DslDeleteArgs,
) -> anyhow::Result<DslDeleteResult> {
  let compiled = load_and_compile(&args.file)?;
  if args.strict && !compiled.warnings.is_empty() {
    return Err(anyhow::anyhow!(
      "dsl strict mode rejected {} warnings",
      compiled.warnings.len()
    ));
  }
  let existing = fetch_workloads(client).await?;
  let managed = existing
    .into_iter()
    .filter(|item| item.name.starts_with(&compiled.prefix))
    .collect::<Vec<_>>();

  let mut stopped = Vec::new();
  for item in managed {
    let path = format!("/tasks/{}/stop", item.id);
    let _: WorkloadSummary = client.post(&path, &serde_json::json!({})).await?;
    stopped.push(item.name);
  }
  stopped.sort();

  Ok(DslDeleteResult {
    application: compiled.application,
    namespace: compiled.namespace,
    stopped,
    warnings: compiled.warnings,
  })
}

pub fn format_plan(plan: &ManifestPlan) -> String {
  format!(
    "application={}/{} create={} keep={} delete_candidates={} warnings={}",
    plan.namespace,
    plan.application,
    plan.create.len(),
    plan.keep.len(),
    plan.delete_candidates.len(),
    plan.warnings.len()
  )
}

fn load_and_compile(file: &str) -> anyhow::Result<CompiledManifest> {
  let manifest = load_manifest(Path::new(file))?;
  compile_manifest(&manifest)
}

async fn fetch_workloads(client: &WardenClient) -> anyhow::Result<Vec<WorkloadSummary>> {
  client.get("/workloads").await
}
