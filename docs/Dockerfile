FROM python:3.11.0a5 as builder

RUN apt update && apt install make -y && pip install --no-cache-dir sphinx==4.4.0 sphinx-rtd-theme==1.0.0

WORKDIR /docs

COPY . .

RUN make html

FROM nginx:1.21-alpine

COPY --from=builder /docs/build/html /usr/share/nginx/html

