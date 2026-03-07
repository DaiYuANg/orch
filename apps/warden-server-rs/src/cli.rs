use clap::Parser;

#[derive(Debug, Parser)]
#[command(name = "warden-server-rs", about = "Warden Rust control plane server")]
pub struct Args {
    #[arg(long = "conf")]
    pub conf: Vec<String>,
}