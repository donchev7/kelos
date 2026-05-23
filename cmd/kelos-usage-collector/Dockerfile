FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
WORKDIR /
COPY bin/kelos-usage-collector-linux-${TARGETARCH} kelos-usage-collector
USER 65532:65532
ENTRYPOINT ["/kelos-usage-collector"]
