use logos::Logos;
use smol_str::SmolStr;
use warden_dsl_core::{DslError, Span};

#[derive(Logos, Debug, Clone, PartialEq, Eq)]
enum RawTokenKind {
  #[token("app")]
  KwApp,
  #[token("let")]
  KwLet,
  #[token("val")]
  KwVal,
  #[token("create")]
  KwCreate,
  #[token("if")]
  KwIf,
  #[token("then")]
  KwThen,
  #[token("else")]
  KwElse,
  #[token("{")]
  LBrace,
  #[token("}")]
  RBrace,
  #[token("(")]
  LParen,
  #[token(")")]
  RParen,
  #[token(",")]
  Comma,
  #[token(".")]
  Dot,
  #[token("=")]
  Eq,
  #[token("==")]
  EqEq,
  #[regex(r#""([^"\\]|\\.)*""#)]
  String,
  #[regex(r"[0-9]+")]
  Number,
  #[regex(r"[A-Za-z_][A-Za-z0-9_]*")]
  Ident,
  #[regex(r"[ \t\r\n\f]+", logos::skip)]
  Whitespace,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Token {
  pub kind: TokenKind,
  pub text: SmolStr,
  pub span: Span,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TokenKind {
  KwApp,
  KwLet,
  KwVal,
  KwCreate,
  KwIf,
  KwThen,
  KwElse,
  LBrace,
  RBrace,
  LParen,
  RParen,
  Comma,
  Dot,
  Eq,
  EqEq,
  String,
  Number,
  Ident,
}

pub fn lex(input: &str) -> Result<Vec<Token>, DslError> {
  let mut lexer = RawTokenKind::lexer(input);
  let mut out = Vec::new();
  while let Some(item) = lexer.next() {
    let range = lexer.span();
    let span = Span::new(range.start, range.end);
    match item {
      Ok(kind) => {
        let kind = map_token(kind);
        out.push(Token {
          kind,
          text: SmolStr::new(lexer.slice()),
          span,
        });
      }
      Err(_) => {
        return Err(DslError::Lex {
          span,
          message: "invalid token".to_string(),
        });
      }
    }
  }
  Ok(out)
}

fn map_token(value: RawTokenKind) -> TokenKind {
  match value {
    RawTokenKind::KwApp => TokenKind::KwApp,
    RawTokenKind::KwLet => TokenKind::KwLet,
    RawTokenKind::KwVal => TokenKind::KwVal,
    RawTokenKind::KwCreate => TokenKind::KwCreate,
    RawTokenKind::KwIf => TokenKind::KwIf,
    RawTokenKind::KwThen => TokenKind::KwThen,
    RawTokenKind::KwElse => TokenKind::KwElse,
    RawTokenKind::LBrace => TokenKind::LBrace,
    RawTokenKind::RBrace => TokenKind::RBrace,
    RawTokenKind::LParen => TokenKind::LParen,
    RawTokenKind::RParen => TokenKind::RParen,
    RawTokenKind::Comma => TokenKind::Comma,
    RawTokenKind::Dot => TokenKind::Dot,
    RawTokenKind::Eq => TokenKind::Eq,
    RawTokenKind::EqEq => TokenKind::EqEq,
    RawTokenKind::String => TokenKind::String,
    RawTokenKind::Number => TokenKind::Number,
    RawTokenKind::Ident | RawTokenKind::Whitespace => TokenKind::Ident,
  }
}
