worker_processes 1;
pid /tmp/nginx.pid;
error_log /dev/null crit;
events { worker_connections 128; }
http {
    access_log off;
    client_body_temp_path /tmp/client-body;
    proxy_temp_path /tmp/proxy-temp;
    fastcgi_temp_path /tmp/fastcgi-temp;
    uwsgi_temp_path /tmp/uwsgi-temp;
    scgi_temp_path /tmp/scgi-temp;
    map "$request_method:$uri" $bridge_allowed {
        default 0;
        ~^GET:/bridge/v1/(capabilities|sources|dataset/lessons)$ 1;
        "~^GET:/bridge/v1/operations/[A-Za-z0-9_-]{1,128}$" 1;
        "~^(GET|HEAD):/bridge/v1/media/[A-Za-z0-9_-]{1,128}$" 1;
        ~^POST:/bridge/v1/(registrations|uploads|extractions|archives|dataset/imports)$ 1;
        "~^POST:/bridge/v1/operations/[A-Za-z0-9_-]{1,128}/(renew|cancel|ack)$" 1;
    }
    server {
        listen {{BRIDGE_BIND_ADDRESS}}:19091;
        server_name _;
        client_max_body_size 1024m;
        location = /_relay/health/live {
            default_type application/json;
            return 200 '{"status":"ok","upstreamVerified":false}';
        }
        location / {
            if ($bridge_allowed = 0) { return 404; }
            proxy_pass http://127.0.0.1:19090;
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header Connection "";
            proxy_request_buffering off;
            proxy_buffering off;
            proxy_max_temp_file_size 0;
            proxy_cache off;
            proxy_store off;
            proxy_ignore_headers X-Accel-Buffering X-Accel-Redirect;
            proxy_hide_header Cache-Control;
            proxy_hide_header Expires;
            add_header Cache-Control "no-store, private, max-age=0" always;
            proxy_connect_timeout 5s;
            proxy_read_timeout 1800s;
            proxy_send_timeout 1800s;
            proxy_next_upstream off;
        }
    }
}
