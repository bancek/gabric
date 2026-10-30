services:
  app:
    build: app
    container_name: app
    restart: unless-stopped
    env_file:
      - .env
    labels:
      reconcile.revision: "{{ .Revision }}"
      traefik.enable: "true"
      traefik.http.routers.app.rule: "Host(`app.{{ .Domain }}`)"
      traefik.http.routers.app.entrypoints: "{{ if .TLS }}websecure{{ else }}web{{ end }}"
{{- if .TLS }}
      traefik.http.routers.app.tls.certresolver: "le"
{{- end }}
      traefik.http.services.app.loadbalancer.server.port: "80"
    networks:
      - web

networks:
  web:
    external: true
