FROM python:3.11-slim
WORKDIR /app
RUN mkdir -p /run/secrets
COPY login.py /app/login.py
ENTRYPOINT ["python", "/app/login.py"]
