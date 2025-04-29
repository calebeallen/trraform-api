

from pymongo.mongo_client import MongoClient
from pymongo.server_api import ServerApi
from dotenv import load_dotenv
import os
import certifi

# Use certifi's CA bundle for SSL verification
ca = certifi.where()


load_dotenv()

# Pull from env like in Go
passw = os.getenv("MONGO_PASSWORD")

# Replace with your connection string
mongo_uri = f"mongodb+srv://caleballen:{passw}@trraform.cenuh0o.mongodb.net/?appName=Trraform"  # or your MongoDB Atlas URI
client = MongoClient(mongo_uri, tlsCAFile=ca)

# Replace with your database and collection name
db = client["Trraform"]
collection = db["users"]


# Count all documents in the collection
count = collection.count_documents({})
print(f"Number of documents in the collection: {count}")
