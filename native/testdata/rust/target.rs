// SPDX-License-Identifier: EPL-2.0
fn classify(data: &[u8]) -> u32 {
    if data == b"A" { return 10; }
    if data == b"B" { return 20; }
    30
}
fn main() {
    let input = std::env::args().nth(1).expect("INPUT path");
    let data = std::fs::read(input).expect("read input");
    std::process::exit(if classify(&data) == 999 { 1 } else { 0 });
}
