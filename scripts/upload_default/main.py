import json
import struct
import boto3
from dotenv import load_dotenv
import os

load_dotenv()
print(os.getenv("CF_R2_API_ENDPOINT"))

# 1. Configuration
R2_ENDPOINT_URL = os.getenv("CF_R2_API_ENDPOINT")
R2_ACCESS_KEY_ID = os.getenv("CF_R2_ACCESS_KEY")
R2_SECRET_ACCESS_KEY = os.getenv("CF_R2_SECRET_KEY")
R2_BUCKET_NAME = 'plots-dev'
OBJECT_NAME = 'default.dat'
LOCAL_DATA_FILE = './default_cactus.dat'

# 2. Generate Payload
with open(LOCAL_DATA_FILE, "rb") as f:
    build_bytes = f.read()

json_bytes = json.dumps({
    "ver": 0, "name": "", "desc": "", "link": "", "linkTitle": ""
}, separators=(",", ":")).encode('utf-8')

out = bytearray(len(json_bytes) + len(build_bytes) + 8)
struct.pack_into("<I", out, 0, len(json_bytes))
out[4:4 + len(json_bytes)] = json_bytes
p = 4 + len(json_bytes)
struct.pack_into("<I", out, p, len(build_bytes))
out[p + 4:p + 4 + len(build_bytes)] = build_bytes
payload = bytes(out)

# 3. Upload to R2
s3_client = boto3.client(
    's3',
    endpoint_url=R2_ENDPOINT_URL,
    aws_access_key_id=R2_ACCESS_KEY_ID,
    aws_secret_access_key=R2_SECRET_ACCESS_KEY,
    region_name='auto'
)

s3_client.put_object(
    Bucket=R2_BUCKET_NAME, 
    Key=OBJECT_NAME, 
    Body=payload,
    ContentType="application/octet-stream"
)

print(f"Successfully uploaded '{OBJECT_NAME}' to bucket '{R2_BUCKET_NAME}'.")


with open("./default_cactus_img.png", "rb") as f:
    s3_client.put_object(
        Bucket="build-images-dev", 
        Key="default.png", 
        Body=f.read(),
        ContentType="image/png"
    )