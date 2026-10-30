services:
  www:
    image: nginx:alpine
    restart: unless-stopped
    volumes:
      - ./conf.d:/etc/nginx/conf.d:ro
      - ./www:/usr/share/nginx/html:ro
    networks:
      - web
    labels:
      traefik.enable: "true"
      traefik.http.routers.www.rule: "Host(`{{ .Domain }}`)"
      traefik.http.routers.www.entrypoints: "{{ if .TLS }}websecure{{ else }}web{{ end }}"
{{- if .TLS }}
      traefik.http.routers.www.tls.certresolver: "le"
{{- end }}
      traefik.http.services.www.loadbalancer.server.port: "80"

networks:
  web:
    external: true
