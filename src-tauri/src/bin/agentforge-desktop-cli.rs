fn main() {
    if let Err(error) = agentforge_desktop_lib::run_standalone_cli(std::env::args().skip(1)) {
        eprintln!("{error}");
        std::process::exit(1);
    }
}
