when HTTP_REQUEST {
    set u [HTTP::uri]
    set n [IP::client_addr]
}

when HTTP_RESPONSE {
    if { [HTTP::status] > 399 & [HTTP::status] < 600 } {
        log "HTTP Error: $n - [HTTP::status] - $u"
        if { [HTTP::status] == 404 } {
            drop
        }
    }
}
