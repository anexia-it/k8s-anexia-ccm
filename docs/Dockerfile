FROM python:3.11.0a5 as builder

RUN apt update && apt install make -y && pip install sphinx sphinx-rtd-theme

WORKDIR /docs

COPY . .

RUN make html

FROM nginx:latest

COPY --from=builder /docs/build/html /usr/share/nginx/html

