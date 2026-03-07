fn main() {
    rhusky::Rhusky::new()
        .hooks_dir("../.githooks")
        .skip_in_env("GITHUB_ACTIONS")
        .with_default_hooks()
        .install()
        .ok();
}