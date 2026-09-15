FROM python:3.12-slim-bookworm
WORKDIR /workspace
COPY python/requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir -r /tmp/requirements.txt && useradd --uid 10001 --create-home vb
COPY python/ /workspace/python/
COPY sql/ /workspace/sql/
COPY data/ /workspace/data/
ENV PYTHONPATH=/workspace/python PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
USER 10001:10001
EXPOSE 8082
CMD ["python","-m","voucherbase.service"]
