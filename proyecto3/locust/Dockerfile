FROM python:3.12-slim

WORKDIR /locust

COPY requirements.txt* ./
RUN pip install --no-cache-dir locust

COPY locustfile.py .

EXPOSE 8089

# HOST must be set via env var: LOCUST_HOST=http://<gateway-ip>
# Example run: docker run -e LOCUST_HOST=http://1.2.3.4 -p 8089:8089 locust
ENTRYPOINT ["locust", "-f", "locustfile.py"]
CMD ["--host", "http://localhost"]
