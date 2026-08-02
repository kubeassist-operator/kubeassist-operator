FROM python:3.12-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application
COPY server.py .
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

# Non-root user for security
RUN useradd -u 1001 -r -g 0 -s /sbin/nologin agent && \
    chown -R 1001:0 /app && \
    chmod -R g=u /app
USER 1001

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
