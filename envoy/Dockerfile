FROM envoyproxy/envoy:v1.34.1
COPY envoy.yaml /etc/envoy/envoy.yaml
COPY descriptor.binpb /etc/envoy/descriptor.binpb
CMD ["-c", "/etc/envoy/envoy.yaml", "--log-level", "info"]
