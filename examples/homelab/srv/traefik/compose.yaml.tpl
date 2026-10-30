services:
  traefik:
    image: traefik:v3
    restart: unless-stopped
    command:
      - --providers.docker=true
      - --providers.docker.network=web
      - --entrypoints.web.address=:80
{{- if .TLS }}
      - --entrypoints.websecure.address=:443
      - --entrypoints.web.http.redirections.entrypoint.to=websecure
      - --certificatesresolvers.le.acme.tlschallenge=true
      - --certificatesresolvers.le.acme.email={{ .Email }}
      - --certificatesresolvers.le.acme.storage=/letsencrypt/acme.json
{{- end }}
    ports:
      - "80:80"
{{- if .TLS }}
      - "443:443"
{{- end }}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
{{- if .TLS }}
      - ./letsencrypt:/letsencrypt
{{- end }}
    networks:
      - web

networks:
  web:
    external: true
