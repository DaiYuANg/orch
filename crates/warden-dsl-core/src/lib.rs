use serde::{Deserialize, Serialize};
use thiserror::Error;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
pub struct Span {
  pub start: usize,
  pub end: usize,
}

impl Span {
  pub const fn new(start: usize, end: usize) -> Self {
    Self { start, end }
  }
}

#[derive(Debug, Error, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum DslError {
  #[error("lex error at {span:?}: {message}")]
  Lex { span: Span, message: String },
  #[error("parse error at {span:?}: {message}")]
  Parse { span: Span, message: String },
}
