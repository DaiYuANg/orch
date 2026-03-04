use crate::cli::E2eArgs;
use crate::util::run_command;
use std::path::Path;

pub fn run_e2e(root: &Path, args: &E2eArgs) -> anyhow::Result<()> {
  run_command(
    root,
    "cargo",
    &[
      "run",
      "-p",
      "warden-cli-rs",
      "--",
      "--api",
      &args.api,
      "workloads",
    ],
  )?;
  run_command(
    root,
    "cargo",
    &[
      "run",
      "-p",
      "warden-cli-rs",
      "--",
      "--api",
      &args.api,
      "routes",
    ],
  )?;
  run_command(
    root,
    "cargo",
    &[
      "run",
      "-p",
      "warden-cli-rs",
      "--",
      "--api",
      &args.api,
      "dns",
    ],
  )?;
  println!("e2e smoke finished");
  Ok(())
}
