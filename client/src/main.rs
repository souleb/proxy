use {
    client::Remote,
    clap::Parser,
};


fn main() {
    let args = Remote::parse();

    match args.connect() {
        Ok(_) => {}
        Err(e) => {
            eprintln!("Error: {e}");
        }
    }
}
