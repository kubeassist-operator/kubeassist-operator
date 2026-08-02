# KubeAssist Operator — Apache License, Version 2.0
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY manager .
USER 65532:65532
ENTRYPOINT ["/manager"]
