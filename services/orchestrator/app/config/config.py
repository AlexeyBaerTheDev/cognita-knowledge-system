import os
from dotenv import load_dotenv

load_dotenv()

def get_database_url(): return os.getenv("DATABASE_URL")
def get_redis_url(): return os.getenv("REDIS_URL")
def get_ollama_url(): return os.getenv("OLLAMA_URL")
def get_embedding_model(): return os.getenv("EMBEDDING_MODEL")
def get_llm_model(): return os.getenv("LLM_MODEL")
def get_upload_dir(): return os.getenv("UPLOAD_DIR", "/data/uploads")