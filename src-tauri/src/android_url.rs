use url::Url;

pub fn validate_server(raw: &str, http_hosts: &[String]) -> Result<Url, String> {
    let url = Url::parse(raw).map_err(|_| {
        "Enter the full Mac library address, including http:// or https:// and its port"
    })?;
    if !["http", "https"].contains(&url.scheme())
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.path() != "/"
        || url.query().is_some()
        || url.fragment().is_some()
    {
        return Err("Use a library address only, without a login, path, query or fragment".into());
    }
    if url.scheme() == "http"
        && !http_hosts
            .iter()
            .any(|host| Some(host.as_str()) == url.host_str())
    {
        return Err("This HTTP host is not included in this APK. Rebuild it for your Mac’s exact host, or use its trusted HTTPS address".into());
    }
    Ok(url)
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn only_exact_http_hosts_and_root_urls() {
        let hosts = vec!["10.0.2.2".into(), "mac.example".into()];
        assert!(validate_server("http://10.0.2.2:4533/", &hosts).is_ok());
        assert!(validate_server("https://mac.example:443", &hosts).is_ok());
        for bad in [
            "http://other.example",
            "http://sub.mac.example",
            "http://mac.example.evil",
            "http://person@mac.example",
            "http://mac.example/path",
            "http://mac.example?x=1",
            "http://mac.example/#x",
            "file:///music",
            "javascript:alert(1)",
            "not a URL",
        ] {
            assert!(validate_server(bad, &hosts).is_err(), "accepted {bad}");
        }
        assert!(validate_server("http://10.0.2.2", &[]).is_err());
    }
}
