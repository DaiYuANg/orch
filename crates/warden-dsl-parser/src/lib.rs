use chumsky::Parser as ChumskyParser;
use chumsky::error::Rich;
use chumsky::prelude::*;
use smol_str::SmolStr;
use warden_dsl_ast::{
  AppDeclAst, BlockAst, CreateDeclAst, DocumentAst, ExprAst, InvokeStmtAst, LetDeclAst, PathAst,
  StmtAst,
};
use warden_dsl_core::{DslError, Span};

pub fn parse(input: &str) -> Result<DocumentAst, DslError> {
  let parser = chumsky_document_parser();
  match parser.parse(input).into_result() {
    Ok(ast) => Ok(ast),
    Err(errs) => {
      let err = errs
        .into_iter()
        .next()
        .unwrap_or_else(|| Rich::custom(chumsky::span::SimpleSpan::new((), 0..0), "invalid dsl"));
      let span = err.span();
      Err(DslError::Parse {
        span: Span::new(span.start, span.end),
        message: err.to_string(),
      })
    }
  }
}

pub fn parse_to_json(input: &str) -> Result<String, DslError> {
  let ast = parse(input)?;
  serde_json::to_string_pretty(&ast).map_err(|err| DslError::Parse {
    span: Span::new(0, 0),
    message: format!("serialize ast failed: {}", err),
  })
}

fn chumsky_document_parser<'src>()
-> impl ChumskyParser<'src, &'src str, DocumentAst, extra::Err<Rich<'src, char>>> {
  let ident = text::ascii::ident()
    .map(|value: &str| SmolStr::new(value))
    .padded();
  let quoted = just('"')
    .ignore_then(none_of('"').repeated().collect::<String>())
    .then_ignore(just('"'))
    .padded();
  let int_value = text::int(10).from_str::<i64>().unwrapped().padded();

  let path = ident
    .separated_by(just('.').padded())
    .at_least(1)
    .collect::<Vec<_>>()
    .map(|segments| PathAst { segments });

  let expr = recursive(|expr| {
    let atom = choice((
      quoted.map(ExprAst::String),
      int_value
        .then_ignore(just('.').padded())
        .then(ident)
        .map(|(value, unit)| ExprAst::MemberNumber { value, unit }),
      int_value.map(ExprAst::Integer),
      path
        .then(
          expr
            .clone()
            .separated_by(just(',').padded())
            .allow_trailing()
            .collect::<Vec<_>>()
            .delimited_by(just('(').padded(), just(')').padded())
            .or_not(),
        )
        .map(|(callee, maybe_args)| {
          if let Some(args) = maybe_args {
            ExprAst::Invocation { callee, args }
          } else if callee.segments.len() == 1 {
            ExprAst::Identifier(callee.segments[0].clone())
          } else {
            ExprAst::Path(callee)
          }
        }),
    ));

    let if_expr = just("if")
      .padded()
      .ignore_then(ident)
      .then_ignore(just("==").padded())
      .then(quoted)
      .then_ignore(just("then").padded())
      .then(atom.clone())
      .then_ignore(just("else").padded())
      .then(atom.clone())
      .map(|(((left, right), then_expr), else_expr)| ExprAst::IfEq {
        left,
        right,
        then_expr: Box::new(then_expr),
        else_expr: Box::new(else_expr),
      });

    choice((if_expr, atom))
  });

  let stmt = recursive(|stmt| {
    let args = expr
      .clone()
      .separated_by(just(',').padded())
      .allow_trailing()
      .collect::<Vec<_>>()
      .delimited_by(just('(').padded(), just(')').padded());
    let body = stmt
      .clone()
      .repeated()
      .collect::<Vec<_>>()
      .delimited_by(just('{').padded(), just('}').padded());

    let let_stmt = just("let")
      .padded()
      .ignore_then(ident)
      .then_ignore(just('=').padded())
      .then(expr.clone())
      .map(|(name, value)| StmtAst::Let(LetDeclAst { name, value }));

    let create_stmt = just("val")
      .padded()
      .ignore_then(ident)
      .then_ignore(just('=').padded())
      .then_ignore(just("create").padded())
      .then(quoted.delimited_by(just('(').padded(), just(')').padded()))
      .then(body.clone())
      .map(|((binding, name), body)| {
        StmtAst::Create(CreateDeclAst {
          binding,
          name,
          body,
        })
      });

    let block_or_invoke = path
      .then(args.clone().or_not())
      .then(body.clone().or_not())
      .try_map(
        |((callee, maybe_args), maybe_body), span| match (maybe_args, maybe_body) {
          (Some(args), Some(body)) => Ok(StmtAst::Block(BlockAst { callee, args, body })),
          (None, Some(body)) => Ok(StmtAst::Block(BlockAst {
            callee,
            args: Vec::new(),
            body,
          })),
          (Some(args), None) => Ok(StmtAst::Invoke(InvokeStmtAst { callee, args })),
          (None, None) => Err(Rich::custom(span, "expected invocation args or block body")),
        },
      );

    choice((let_stmt, create_stmt, block_or_invoke))
  });

  just("app")
    .padded()
    .ignore_then(quoted.delimited_by(just('(').padded(), just(')').padded()))
    .then(
      stmt
        .repeated()
        .collect::<Vec<_>>()
        .delimited_by(just('{').padded(), just('}').padded()),
    )
    .map(|(name, body)| DocumentAst {
      app: AppDeclAst { name, body },
    })
    .then_ignore(end())
}

#[cfg(test)]
mod tests {
  use super::parse;

  #[test]
  fn parses_full_sample_shape() {
    let raw = r#"
app("mall") {
  let env = "prod"
  let version = "1.2.3"
  services {
    val redis = create("redis") {
      runtime(container)
      image("redis:7")
      expose("redis") { container(6379) }
    }
    val gateway = create("gateway") {
      runtime(container)
      image("ghcr.io/acme/gateway:${version}")
      replicas(if env == "prod" then 3 else 1)
      dependsOn(redis)
      env { set("REDIS_ADDR", redis.endpoint("redis")) }
      resources { cpu(500.milli) memory(512.Mi) }
      healthcheck { http { path("/health") port(http) } }
    }
  }
  ingress("gateway-public") {
    host("mall.example.com")
    route("/") { backend(services.gateway) port("http") }
  }
}
"#;

    let ast = parse(raw).expect("parse dsl");
    assert_eq!(ast.app.name, "mall");
    assert_eq!(ast.app.body.len(), 4);
  }
}
