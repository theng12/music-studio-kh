use serde_json::{json, Value};
use std::io::{BufRead, BufReader, Read, Write};
use std::path::PathBuf;
use std::process::{Child, ChildStdin, ChildStdout, Command, Stdio};

pub fn parse_response(id: u64, line: &str) -> Result<Value, String> {
    let value: Value =
        serde_json::from_str(line).map_err(|_| "Invalid response from music server".to_string())?;
    if value["id"].as_u64() != Some(id) {
        return Err("Music server response did not match request".into());
    }
    if let Some(error) = value["error"].as_str() {
        if !error.is_empty() {
            return Err(error.to_string());
        }
    }
    Ok(value["result"].clone())
}
pub fn validate_profile_url(raw: &str) -> Result<url::Url, String> {
    let parsed = url::Url::parse(raw).map_err(|_| "Invalid profile address".to_string())?;
    if parsed.scheme() != "http"
        || parsed.host_str() != Some("127.0.0.1")
        || parsed.port().map(|p| p < 1024).unwrap_or(true)
        || !parsed.username().is_empty()
        || parsed.password().is_some()
        || parsed.query().is_some()
        || parsed.fragment().is_some()
        || parsed.path() != "/"
    {
        return Err("Profile windows must use a local server address".into());
    }
    Ok(parsed)
}
pub struct Bridge {
    child: Child,
    input: Option<ChildStdin>,
    output: BufReader<ChildStdout>,
    next_id: u64,
}
impl Bridge {
    pub fn spawn(executable: PathBuf, tools: PathBuf, data: PathBuf) -> Result<Self, String> {
        let mut command = Command::new(executable);
        command
            .args(["--data"])
            .arg(data)
            .arg("desktop")
            .arg("--tools")
            .arg(tools)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::inherit());
        #[cfg(windows)]
        {
            use std::os::windows::process::CommandExt;
            command.creation_flags(0x08000000);
        }
        let mut child = command.spawn().map_err(|_| {
            "Music Studio KH’s server could not start. Reinstall the app or check its permissions."
                .to_string()
        })?;
        let input = child
            .stdin
            .take()
            .ok_or("Could not connect to music server")?;
        let output = child
            .stdout
            .take()
            .ok_or("Could not read music server response")?;
        Ok(Self {
            child,
            input: Some(input),
            output: BufReader::new(output),
            next_id: 0,
        })
    }
    pub fn request(&mut self, method: &str, params: Value) -> Result<Value, String> {
        self.next_id += 1;
        let request = json!({"id":self.next_id,"method":method,"params":params});
        let encoded = serde_json::to_string(&request).map_err(|_| "Invalid desktop request")?;
        if encoded.len() > 1024 * 1024 {
            return Err("Desktop request is too large".into());
        }
        let input = self.input.as_mut().ok_or("Music server is stopping")?;
        writeln!(input, "{}", encoded).map_err(|_| "Music server disconnected")?;
        input.flush().map_err(|_| "Music server disconnected")?;
        let mut line = String::new();
        self.output
            .by_ref()
            .take(1024 * 1024 + 1)
            .read_line(&mut line)
            .map_err(|_| "Music server disconnected")?;
        if line.len() > 1024 * 1024 || !line.ends_with('\n') {
            return Err("Music server response is too large".into());
        }
        parse_response(self.next_id, &line)
    }
    pub fn shutdown(&mut self) {
        let _ = self.request("quit", json!({}));
        self.input.take();
        let _ = self.child.wait();
    }
}
impl Drop for Bridge {
    fn drop(&mut self) {
        self.input.take();
        // EOF requests the Go supervisor to stop all owned children before exiting.
        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(20);
        while self.child.try_wait().ok().flatten().is_none() && std::time::Instant::now() < deadline
        {
            std::thread::sleep(std::time::Duration::from_millis(25));
        }
        if self.child.try_wait().ok().flatten().is_none() {
            let _ = self.child.kill();
        }
        let _ = self.child.wait();
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn response_id_and_error_are_checked() {
        assert_eq!(
            parse_response(7, r#"{"id":7,"result":{"ok":true}}"#).unwrap()["ok"],
            true
        );
        assert!(parse_response(7, r#"{"id":8,"result":{}}"#).is_err());
        assert!(parse_response(7, r#"{"id":7,"error":"failed"}"#).is_err());
    }
    #[test]
    fn profile_windows_are_loopback_only() {
        assert!(validate_profile_url("http://127.0.0.1:4747/").is_ok());
        for raw in [
            "https://evil.example/",
            "http://127.0.0.1.evil.example:4747/",
            "http://user:password@127.0.0.1:4747/",
            "http://127.0.0.1:4747/?token=secret",
        ] {
            assert!(validate_profile_url(raw).is_err());
        }
    }
}
