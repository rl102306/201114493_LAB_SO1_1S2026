import random
from datetime import datetime, timezone

from locust import HttpUser, task, between


COUNTRIES = ["USA", "RUS", "CHN", "ESP", "GTM"]
ENDPOINT = "/grpc-201114493"


class WarReportUser(HttpUser):
    """Simulates a client sending military reports to the API gateway."""

    wait_time = between(0.1, 0.5)

    @task
    def send_war_report(self):
        payload = {
            "country": random.choice(COUNTRIES),
            "warplanes_in_air": random.randint(0, 50),
            "warships_in_water": random.randint(0, 30),
            "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        }

        with self.client.post(
            ENDPOINT,
            json=payload,
            catch_response=True,
            name=ENDPOINT,
        ) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(
                    f"Unexpected status {response.status_code}: {response.text[:200]}"
                )

    @task(weight=5)
    def send_rus_report(self):
        """Weighted task: heavier traffic for the assigned country (RUS)."""
        payload = {
            "country": "RUS",
            "warplanes_in_air": random.randint(0, 50),
            "warships_in_water": random.randint(0, 30),
            "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        }

        with self.client.post(
            ENDPOINT,
            json=payload,
            catch_response=True,
            name=f"{ENDPOINT}[RUS]",
        ) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(
                    f"Unexpected status {response.status_code}: {response.text[:200]}"
                )
