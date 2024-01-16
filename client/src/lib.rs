use std::io::{BufRead, Write};

use {
  std::net::TcpStream,
  std::io::{self, BufReader},
  std::thread,
  clap::Parser,
};


/// Remote is a command line tool to connect to a remote server
/// and send data and receive data.
#[derive(Parser)]
pub struct Remote {

    /// The address to connect to.
    #[arg(short, long, default_value_t = String::from("127.0.0.1"))]
    pub address: String,

    /// The port to connect to.
    #[arg(short, long)]
    pub port: u16,
  }

impl Remote {
  /// Connect to the remote server and send and receive data.
  /// This function will block until the connection is closed.
  /// Errors:
  /// - io::Error
  pub fn connect(self) -> io::Result<()> {
    println!("{}:{}", self.address, self.port);
    let mut stream = TcpStream::connect(format!("{}:{}", self.address, self.port))?;
    let mut stream2 = stream.try_clone()?;

    // spawn a thread to read from stdin and write to the stream
    thread::spawn(move ||{
      let mut reader = BufReader::new(io::stdin());
      let mut buf = String::new();
      loop {
        match reader.read_line(&mut buf) {
          Ok(0) => {
            // close the write half of the strream
            stream.shutdown(std::net::Shutdown::Write)?;
            return Ok(())
          }
          Ok(_) => {
            stream.write_all(&buf.as_bytes())?;
          }
          Err(e) => {
            return Err(e);
          }
        }
        buf.clear();
      }
    });

    let mut reader = io::BufReader::new(&mut stream2);
    let mut buf_len: usize;
    loop {
      match reader.fill_buf() {
          Ok([]) => return Ok(()),
          Ok(buf) => {
            buf_len = buf.len();
            io::stdout().write_all(&buf[..buf_len])?;
          }
          Err(e) => {
            return Err(e);
          }
      }
      reader.consume(buf_len);
    }
  }
}
