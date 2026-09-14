# Two stages: build with the toolchain, ship without it.
#
# The result is a single static binary on a distroless base - about 20MB, against roughly
# 150MB for the Node image it sits beside. That difference is most of the memory argument for
# doing this at all, so it is worth not giving it away with a fat base image.

FROM golang:1.25-alpine AS build
WORKDIR /src

# Dependencies first, as their own layer: they change far less often than the code, so a
# normal edit reuses the cached download instead of refetching the module graph.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 makes it static, which is what lets the final stage have no libc at all.
# -trimpath keeps build-machine paths out of the binary; -s -w drop the symbol table and DWARF,
# roughly a third of the size.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Tests run in the image build too, so a broken build cannot produce a runnable image.
RUN go vet ./... && go test ./...

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
# Non-root by default. There is no shell in this image, so an RCE has nothing to exec.
USER nonroot:nonroot
EXPOSE 5002
ENTRYPOINT ["/api"]
