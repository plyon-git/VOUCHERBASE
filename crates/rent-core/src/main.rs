// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
use std::io::{self, Read};
fn main() {
    let mut s = String::new();
    io::stdin().take(1_048_577).read_to_string(&mut s).unwrap();
    if s.len() > 1_048_576 {
        eprintln!("Input too large");
        std::process::exit(2);
    }
    let result = serde_json::from_str::<rent_core::Request>(&s)
        .map_err(|e| e.to_string())
        .and_then(|r| rent_core::calculate(&r));
    match result {
        Ok(v) => println!("{}", serde_json::to_string(&v).unwrap()),
        Err(e) => {
            eprintln!("{e}");
            std::process::exit(2);
        }
    }
}
