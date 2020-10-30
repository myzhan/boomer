local http = require("http")

function execute()
    local start = get_millis()
    local resp = http.get("http://localhost:8000")
    local elapsed = get_millis() - start
    if not resp.error then
        record_success("http", "get", elapsed, #resp.body)
    else
        record_failure("http", "get", elapsed, "empty body")
    end
end