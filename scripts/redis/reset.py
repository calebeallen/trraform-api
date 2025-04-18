import redis
import os
from dotenv import load_dotenv

load_dotenv()

# Pull from env like in Go
redis_password = os.getenv("REDIS_PASSWORD")

r = redis.Redis(
    host='redis-16216.c15.us-east-1-4.ec2.redns.redis-cloud.com',
    port=16216,
    username='default',
    password=redis_password,
    db=0,
)

r.flushall()
print("Redis database flushed.")

plot_ids = [hex(i)[2:] for i in range(1, 34999)]

key = "openplots:0"
r.sadd(key, *plot_ids)
print("Depth 0 plots added")

