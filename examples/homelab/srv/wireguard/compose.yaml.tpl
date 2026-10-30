services:
  wg-easy:
    image: ghcr.io/wg-easy/wg-easy
    restart: unless-stopped
    environment:
      - WG_HOST=vpn.example.net
      - WG_PORT=51820
    volumes:
      - ./data:/etc/wireguard
      - /lib/modules:/lib/modules:ro
    ports:
      - "51820:51820/udp"
      - "127.0.0.1:51821:51821/tcp"
    cap_add:
      - NET_ADMIN
      - SYS_MODULE
    sysctls:
      - net.ipv4.ip_forward=1
      - net.ipv4.conf.all.src_valid_mark=1
{{- if .IPv6 }}
      - net.ipv6.conf.all.forwarding=1
{{- end }}
