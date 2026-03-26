use ariadne::{Color, Label, Report, ReportKind, Source};
use serde::Serialize;
use std::path::{Path, PathBuf};
use thiserror::Error;
use warden_dsl_ast::{AppDeclAst, DocumentAst, PathAst, StmtAst};
use warden_dsl_core::DslError;

#[derive(Debug, Serialize)]
pub struct PlannerOutput {
  pub ast: warden_dsl_ast::DocumentAst,
  pub hir: warden_dsl_hir::HirDocument,
  pub bound: warden_dsl_binder::BoundDocument,
  pub ir: warden_dsl_ir::IrApplication,
  pub canonical: warden_dsl_canonical::CanonicalApplication,
  pub apply: warden_dsl_canonical::CanonicalApplyOutput,
  pub creation_order: Vec<String>,
}

#[derive(Debug, Error)]
pub enum PlannerError {
  #[error("parse error in {file}: {err}")]
  ParseFile {
    file: String,
    input: String,
    err: DslError,
  },
  #[error("{0}")]
  Hir(String),
  #[error("{0}")]
  Semantic(String),
  #[error("{0}")]
  Bind(String),
  #[error("{0}")]
  Canonical(String),
  #[error("{0}")]
  Io(String),
  #[error("{0}")]
  Import(String),
}

pub fn plan_source(raw: &str) -> Result<PlannerOutput, PlannerError> {
  let ast = warden_dsl_parser::parse(raw).map_err(|err| PlannerError::ParseFile {
    file: String::from("<inline>"),
    input: raw.to_string(),
    err,
  })?;
  plan_ast(ast)
}

pub fn plan_file(path: &Path) -> Result<PlannerOutput, PlannerError> {
  let ast = load_document_with_imports(path, &mut Vec::new())?;
  plan_ast(ast)
}

fn plan_ast(ast: DocumentAst) -> Result<PlannerOutput, PlannerError> {
  let hir = warden_dsl_hir::lower(&ast).map_err(|err| PlannerError::Hir(err.to_string()))?;
  let sema =
    warden_dsl_sema::analyze_hir(&hir).map_err(|err| PlannerError::Semantic(err.to_string()))?;
  let bound = warden_dsl_binder::bind(&hir).map_err(|err| PlannerError::Bind(err.to_string()))?;
  let ir = warden_dsl_ir::lower(&hir);
  let canonical =
    warden_dsl_canonical::lower(&hir).map_err(|err| PlannerError::Canonical(err.to_string()))?;
  let apply = warden_dsl_canonical::compile_apply_output(&canonical);
  Ok(PlannerOutput {
    ast,
    hir,
    bound,
    ir,
    canonical,
    apply,
    creation_order: sema.creation_order,
  })
}

fn load_document_with_imports(
  path: &Path,
  stack: &mut Vec<PathBuf>,
) -> Result<DocumentAst, PlannerError> {
  let canonical = canonicalize_existing(path)?;
  ensure_not_in_stack(&canonical, stack)?;
  stack.push(canonical.clone());

  let source = std::fs::read_to_string(&canonical).map_err(|err| {
    PlannerError::Io(format!(
      "read dsl file {} failed: {}",
      canonical.display(),
      err
    ))
  })?;
  let parsed = warden_dsl_parser::parse(&source).map_err(|err| PlannerError::ParseFile {
    file: canonical.display().to_string(),
    input: source.clone(),
    err,
  })?;
  let base_dir = canonical.parent().unwrap_or_else(|| Path::new("."));
  let body = resolve_imports(parsed.app.body, base_dir, stack)?;

  stack.pop();
  Ok(DocumentAst {
    app: AppDeclAst {
      name: parsed.app.name,
      body,
    },
  })
}

fn load_fragment_with_imports(
  path: &Path,
  stack: &mut Vec<PathBuf>,
) -> Result<Vec<StmtAst>, PlannerError> {
  let canonical = canonicalize_existing(path)?;
  ensure_not_in_stack(&canonical, stack)?;
  stack.push(canonical.clone());

  let source = std::fs::read_to_string(&canonical).map_err(|err| {
    PlannerError::Io(format!(
      "read dsl fragment {} failed: {}",
      canonical.display(),
      err
    ))
  })?;
  let parsed =
    warden_dsl_parser::parse_fragment(&source).map_err(|err| PlannerError::ParseFile {
      file: canonical.display().to_string(),
      input: source.clone(),
      err,
    })?;
  validate_fragment_statements(&parsed, &canonical)?;
  let base_dir = canonical.parent().unwrap_or_else(|| Path::new("."));
  let body = resolve_imports(parsed, base_dir, stack)?;

  stack.pop();
  Ok(body)
}

fn resolve_imports(
  statements: Vec<StmtAst>,
  base_dir: &Path,
  stack: &mut Vec<PathBuf>,
) -> Result<Vec<StmtAst>, PlannerError> {
  let mut out = Vec::new();
  for stmt in statements {
    match stmt {
      StmtAst::Import(value) => {
        let target = base_dir.join(&value.path);
        let imported = load_fragment_with_imports(&target, stack)?;
        out.extend(imported);
      }
      other => {
        reject_nested_special_forms(&other)?;
        out.push(other);
      }
    }
  }
  Ok(out)
}

fn validate_fragment_statements(statements: &[StmtAst], path: &Path) -> Result<(), PlannerError> {
  for stmt in statements {
    if is_app_statement(stmt) {
      return Err(PlannerError::Import(format!(
        "imported fragment {} must not declare app(...)",
        path.display()
      )));
    }
  }
  Ok(())
}

fn reject_nested_special_forms(stmt: &StmtAst) -> Result<(), PlannerError> {
  match stmt {
    StmtAst::Import(value) => Err(PlannerError::Import(format!(
      "import({}) is only allowed at the top level of app bodies and imported fragments",
      value.path
    ))),
    StmtAst::Create(value) => {
      for nested in &value.body {
        reject_nested_special_forms(nested)?;
      }
      Ok(())
    }
    StmtAst::Block(value) => {
      if path_is_single(&value.callee, "app") {
        return Err(PlannerError::Import(String::from(
          "nested app(...) declarations are not allowed",
        )));
      }
      for nested in &value.body {
        reject_nested_special_forms(nested)?;
      }
      Ok(())
    }
    StmtAst::Let(_) | StmtAst::Invoke(_) => Ok(()),
  }
}

fn is_app_statement(stmt: &StmtAst) -> bool {
  matches!(stmt, StmtAst::Block(value) if path_is_single(&value.callee, "app"))
}

fn path_is_single(path: &PathAst, want: &str) -> bool {
  path.segments.len() == 1 && path.segments[0].as_str() == want
}

fn ensure_not_in_stack(path: &Path, stack: &[PathBuf]) -> Result<(), PlannerError> {
  if let Some(idx) = stack.iter().position(|item| item == path) {
    let cycle = stack[idx..]
      .iter()
      .chain(std::iter::once(&path.to_path_buf()))
      .map(|item| item.display().to_string())
      .collect::<Vec<_>>()
      .join(" -> ");
    return Err(PlannerError::Import(format!(
      "import cycle detected: {}",
      cycle
    )));
  }
  Ok(())
}

fn canonicalize_existing(path: &Path) -> Result<PathBuf, PlannerError> {
  std::fs::canonicalize(path).map_err(|err| {
    PlannerError::Io(format!(
      "resolve dsl path {} failed: {}",
      path.display(),
      err
    ))
  })
}

pub fn print_diagnostic(file: &str, source: &str, err: &DslError) {
  let (span, message) = match err {
    DslError::Lex { span, message } | DslError::Parse { span, message } => (span, message),
  };
  let report_span = (file, span.start..span.end.max(span.start + 1));
  let _ = Report::build(ReportKind::Error, report_span.clone())
    .with_message(message)
    .with_label(
      Label::new(report_span)
        .with_message(message)
        .with_color(Color::Red),
    )
    .finish()
    .print((file, Source::from(source)));
}

#[cfg(test)]
mod tests {
  use super::plan_file;
  use std::fs;
  use tempfile::tempdir;

  #[test]
  fn plan_file_expands_imported_fragments() {
    let dir = tempdir().expect("temp dir");
    let main = dir.path().join("main.wd");
    let modules = dir.path().join("modules");
    fs::create_dir_all(&modules).expect("create modules");
    fs::write(
      modules.join("redis.wd"),
      r#"
services {
  val redis = create("redis") {
    runtime(containerd)
  }
}
"#,
    )
    .expect("write fragment");
    fs::write(
      &main,
      r#"
app("mall") {
  import("./modules/redis.wd")
  services {
    val gateway = create("gateway") {
      runtime(container)
      dependsOn(redis)
    }
  }
}
"#,
    )
    .expect("write main");

    let output = plan_file(&main).expect("plan file");
    assert_eq!(output.ast.app.body.len(), 2);
    assert_eq!(output.canonical.workloads.len(), 2);
    assert_eq!(
      output.creation_order,
      vec![String::from("redis"), String::from("gateway")]
    );
    assert_eq!(output.apply.application, "mall");
  }

  #[test]
  fn plan_file_rejects_import_cycles() {
    let dir = tempdir().expect("temp dir");
    let main = dir.path().join("main.wd");
    let a = dir.path().join("a.wd");
    let b = dir.path().join("b.wd");
    fs::write(
      &main,
      r#"
app("mall") {
  import("./a.wd")
}
"#,
    )
    .expect("write main");
    fs::write(
      &a,
      r#"
import("./b.wd")
"#,
    )
    .expect("write a");
    fs::write(
      &b,
      r#"
import("./a.wd")
"#,
    )
    .expect("write b");

    let err = plan_file(&main).expect_err("should reject cycle");
    assert!(err.to_string().contains("import cycle detected"));
  }

  #[test]
  fn plan_file_supports_workload_and_endpoint_syntax() {
    let dir = tempdir().expect("temp dir");
    let main = dir.path().join("main.wd");
    let modules = dir.path().join("modules");
    fs::create_dir_all(&modules).expect("create modules");
    fs::write(
      modules.join("redis.wd"),
      r#"
workload("redis") {
  kind(stateful)
  runtime(containerd)
  endpoint("redis") { port(6379) protocol(tcp) }
}
"#,
    )
    .expect("write fragment");
    fs::write(
      &main,
      r#"
app("mall") {
  import("./modules/redis.wd")
  config("appConfig") {}
  secret("dbPassword") {}
  volume("redisData") {}
  workload("gateway") {
    kind(worker)
    runtime(container)
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
"#,
    )
    .expect("write main");

    let output = plan_file(&main).expect("plan file");
    assert_eq!(output.hir.services.len(), 2);
    assert_eq!(output.hir.configs[0].name, "appConfig");
    assert_eq!(output.hir.secrets[0].name, "dbPassword");
    assert_eq!(output.hir.volumes[0].name, "redisData");
    assert_eq!(output.bound.volumes[0].name, "redisData");
    assert_eq!(output.bound.configs[0].name, "appConfig");
    assert_eq!(output.bound.secrets[0].name, "dbPassword");
    assert_eq!(output.bound.workloads[0].kind.as_deref(), Some("stateful"));
    assert_eq!(output.bound.workloads[0].service_port, Some(6379));
    assert_eq!(output.bound.workloads[1].kind.as_deref(), Some("worker"));
    assert_eq!(output.bound.workloads[1].mounts[0].volume.name, "redisData");
    assert_eq!(output.bound.workloads[1].env.len(), 4);
    assert_eq!(
      output.bound.workloads[1]
        .resources
        .as_ref()
        .and_then(|value| value.cpu.as_deref()),
      Some("500.milliCpu")
    );
    assert_eq!(
      output.bound.workloads[1]
        .health
        .as_ref()
        .and_then(|value| value.readiness.as_ref())
        .map(|value| value.path.as_str()),
      Some("/ready")
    );
    assert_eq!(
      output.bound.workloads[1].endpoint_name.as_deref(),
      Some("http")
    );
    assert_eq!(
      output.bound.workloads[1].endpoint_protocol.as_deref(),
      Some("http")
    );
    assert_eq!(
      output.canonical.workloads[0].kind,
      warden_dsl_canonical::WorkloadKind::Stateful
    );
    assert_eq!(
      output.canonical.workloads[1].kind,
      warden_dsl_canonical::WorkloadKind::Worker
    );
    assert_eq!(output.canonical.configs[0].name, "appConfig");
    assert_eq!(output.canonical.secrets[0].name, "dbPassword");
    assert_eq!(output.canonical.volumes[0].name, "redisData");
    assert_eq!(
      output.canonical.workloads[1].mounts[0].volume.name,
      "redisData"
    );
    assert_eq!(output.canonical.workloads[1].run.env.len(), 4);
    assert_eq!(
      output.canonical.workloads[1]
        .run
        .resources
        .as_ref()
        .and_then(|value| value.cpu_millis),
      Some(500)
    );
    assert_eq!(
      output.canonical.workloads[1]
        .health
        .as_ref()
        .and_then(|value| value.readiness.as_ref())
        .map(|value| value.path.as_str()),
      Some("/ready")
    );
    assert_eq!(output.canonical.workloads[1].depends_on[0].name, "redis");
    assert_eq!(
      output.canonical.ingresses[0].routes[0].backend.workload,
      "gateway"
    );
    assert_eq!(output.apply.ingress_routes.len(), 1);
    assert_eq!(output.apply.ingress_routes[0].backend.workload, "gateway");
  }
}
