use anyhow::Context;
use ariadne::{Color, Label, Report, ReportKind, Source};
use clap::Parser;
use serde::Serialize;
use warden_dsl_core::DslError;

#[derive(Debug, Parser)]
#[command(name = "warden-dsl-planner")]
struct Args {
  /// Path to invocation DSL file
  #[arg(long)]
  file: String,
}

#[derive(Debug, Serialize)]
struct PlannerOutput {
  ast: warden_dsl_ast::DocumentAst,
  hir: warden_dsl_hir::HirDocument,
  ir: warden_dsl_ir::IrApplication,
  creation_order: Vec<String>,
}

fn main() -> anyhow::Result<()> {
  let args = Args::parse();
  let raw =
    std::fs::read_to_string(&args.file).with_context(|| format!("read dsl file {}", args.file))?;
  let ast = match warden_dsl_parser::parse(&raw) {
    Ok(value) => value,
    Err(err) => {
      print_diagnostic(&args.file, &raw, &err);
      return Err(anyhow::anyhow!("{}", err));
    }
  };
  let hir = warden_dsl_hir::lower(&ast).map_err(|err| anyhow::anyhow!("{}", err))?;
  let sema = warden_dsl_sema::analyze_hir(&hir).map_err(|err| anyhow::anyhow!("{}", err))?;
  let ir = warden_dsl_ir::lower(&hir);
  let output = PlannerOutput {
    ast,
    hir,
    ir,
    creation_order: sema.creation_order,
  };
  println!("{}", serde_json::to_string_pretty(&output)?);
  Ok(())
}

fn print_diagnostic(file: &str, source: &str, err: &DslError) {
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
