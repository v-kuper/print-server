# ATOL Linux x64 Driver Drop-In

This folder is used by the default Docker target `runtime-plain`.

Current contents were copied from:

```text
10.10.7.0/linux-x64/*.so*
```

The required file is:

```text
libfptr10.so
```

Neighboring `.so*` files are copied too so the driver can resolve its bundled runtime dependencies in Docker.

Rebuild after replacing these files:

```bash
cd /Users/vitalikupratsevich/Documents/home/atol/server
docker compose build --no-cache
docker compose up -d
```

This target does not start UEMA and uses this folder as `ATOL_LIBRARY_PATH=/opt/atol/lib`.
