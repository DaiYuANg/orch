use serde::{Deserialize, Serialize};
use smol_str::SmolStr;
use std::fmt;

#[repr(u16)]
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum SyntaxKind {
  Document,
  AppDecl,
  Stmt,
  Expr,
  Token,
}

impl From<SyntaxKind> for rowan::SyntaxKind {
  fn from(value: SyntaxKind) -> Self {
    rowan::SyntaxKind(value as u16)
  }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum DslLanguage {}

impl rowan::Language for DslLanguage {
  type Kind = SyntaxKind;

  fn kind_from_raw(raw: rowan::SyntaxKind) -> Self::Kind {
    match raw.0 {
      0 => SyntaxKind::Document,
      1 => SyntaxKind::AppDecl,
      2 => SyntaxKind::Stmt,
      3 => SyntaxKind::Expr,
      _ => SyntaxKind::Token,
    }
  }

  fn kind_to_raw(kind: Self::Kind) -> rowan::SyntaxKind {
    kind.into()
  }
}

pub type SyntaxNode = rowan::SyntaxNode<DslLanguage>;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DocumentAst {
  pub app: AppDeclAst,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct AppDeclAst {
  pub name: String,
  pub body: Vec<StmtAst>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum StmtAst {
  Let(LetDeclAst),
  Import(ImportStmtAst),
  Create(CreateDeclAst),
  Block(BlockAst),
  Invoke(InvokeStmtAst),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LetDeclAst {
  pub name: SmolStr,
  pub value: ExprAst,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ImportStmtAst {
  pub path: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateDeclAst {
  pub binding: SmolStr,
  pub name: String,
  pub body: Vec<StmtAst>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct BlockAst {
  pub callee: PathAst,
  pub args: Vec<ExprAst>,
  pub body: Vec<StmtAst>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct InvokeStmtAst {
  pub callee: PathAst,
  pub args: Vec<ExprAst>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct PathAst {
  pub segments: Vec<SmolStr>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum ExprAst {
  String(String),
  Integer(i64),
  Identifier(SmolStr),
  Path(PathAst),
  MemberNumber {
    value: i64,
    unit: SmolStr,
  },
  IfEq {
    left: SmolStr,
    right: String,
    then_expr: Box<ExprAst>,
    else_expr: Box<ExprAst>,
  },
  Invocation {
    callee: PathAst,
    args: Vec<ExprAst>,
  },
}

pub fn to_rowan_green(doc: &DocumentAst) -> rowan::GreenNode {
  let mut builder = rowan::GreenNodeBuilder::new();
  builder.start_node(SyntaxKind::Document.into());
  push_app(&mut builder, &doc.app);
  builder.finish_node();
  builder.finish()
}

fn push_app(builder: &mut rowan::GreenNodeBuilder<'_>, app: &AppDeclAst) {
  builder.start_node(SyntaxKind::AppDecl.into());
  token(builder, "app");
  token(builder, "(");
  token(builder, &format!("{:?}", app.name));
  token(builder, ")");
  token(builder, "{");
  for stmt in &app.body {
    push_stmt(builder, stmt);
  }
  token(builder, "}");
  builder.finish_node();
}

fn push_stmt(builder: &mut rowan::GreenNodeBuilder<'_>, stmt: &StmtAst) {
  builder.start_node(SyntaxKind::Stmt.into());
  match stmt {
    StmtAst::Let(value) => {
      token(builder, "let");
      token(builder, value.name.as_str());
      push_expr(builder, &value.value);
    }
    StmtAst::Import(value) => {
      token(builder, "import");
      token(builder, "(");
      token(builder, &format!("{:?}", value.path));
      token(builder, ")");
    }
    StmtAst::Create(value) => {
      token(builder, "val");
      token(builder, value.binding.as_str());
      token(builder, "=");
      token(builder, "create");
      token(builder, &format!("{:?}", value.name));
      for nested in &value.body {
        push_stmt(builder, nested);
      }
    }
    StmtAst::Block(value) => {
      token(builder, &path_text(&value.callee));
      for expr in &value.args {
        push_expr(builder, expr);
      }
      for nested in &value.body {
        push_stmt(builder, nested);
      }
    }
    StmtAst::Invoke(value) => {
      token(builder, &path_text(&value.callee));
      for expr in &value.args {
        push_expr(builder, expr);
      }
    }
  }
  builder.finish_node();
}

fn push_expr(builder: &mut rowan::GreenNodeBuilder<'_>, expr: &ExprAst) {
  builder.start_node(SyntaxKind::Expr.into());
  token(builder, &format_expr(expr));
  builder.finish_node();
}

fn format_expr(expr: &ExprAst) -> String {
  match expr {
    ExprAst::String(value) => format!("{:?}", value),
    ExprAst::Integer(value) => value.to_string(),
    ExprAst::Identifier(value) => value.to_string(),
    ExprAst::Path(value) => path_text(value),
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
      format_expr(then_expr),
      format_expr(else_expr)
    ),
    ExprAst::Invocation { callee, args } => {
      let mut out = String::new();
      let _ = fmt::write(&mut out, format_args!("{}(", path_text(callee)));
      for (idx, arg) in args.iter().enumerate() {
        if idx > 0 {
          out.push_str(", ");
        }
        out.push_str(&format_expr(arg));
      }
      out.push(')');
      out
    }
  }
}

fn path_text(path: &PathAst) -> String {
  path
    .segments
    .iter()
    .map(ToString::to_string)
    .collect::<Vec<_>>()
    .join(".")
}

fn token(builder: &mut rowan::GreenNodeBuilder<'_>, text: &str) {
  builder.token(SyntaxKind::Token.into(), text);
}
