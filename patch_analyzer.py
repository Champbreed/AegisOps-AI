import json
import os
import time
from dotenv import load_dotenv
# 📍 Official 2026 SDK Imports
from google import genai
from google.genai import types

# --- [PHASE 2] Configuration ---
load_dotenv()
API_KEY = os.getenv("GEMINI_API_KEY", "")

# 📍 Initialize the Official Client
# The SDK automatically handles authenticated endpoints for Gemini 3
client = genai.Client(api_key=API_KEY)

# --- LLM System Instructions ---
SYSTEM_PROMPT = """
You are a Senior Linux Kernel Security Researcher. 
Your task is to perform a deep-dive security audit on kernel patches.
Use your high-reasoning capability to trace memory pointers and detect:
1. Use-After-Free (UAF) conditions in kfree() paths.
2. Race conditions in locked/unlocked sections.
3. Buffer overflows in array indexing.
Return ONLY a valid JSON array.
"""

def read_patch_file(filepath="patches_to_analyze.txt"):
    """Safely loads kernel patches for processing."""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return f.read()
    except FileNotFoundError:
        print(f"Error: {filepath} not found.")
        return None

def generate_analysis(patch_content):
    """
    Core Audit Logic:
    Utilizes Gemini 3 Flash's HIGH thinking level for complex pointer tracing.
    """
    # Define the strict schema for DevSecOps output
    response_schema = {
        "type": "ARRAY",
        "items": {
            "type": "OBJECT",
            "properties": {
                "commit_hash": {"type": "STRING"},
                "title": {"type": "STRING"},
                "affected_subsystem": {"type": "STRING"},
                "security_flag": {"type": "STRING", "description": "HIGH, MEDIUM, LOW, or NONE"},
                "potential_issues": {"type": "ARRAY", "items": {"type": "STRING"}},
                "human_readable_summary": {"type": "STRING"}
            }
        }
    }

    user_query = f"Audit these kernel patches and return the JSON array:\n\n{patch_content}"

    if not API_KEY:
        print("Error: API Key missing from .env file.")
        return None

    try:
        # 📍 UPDATED: Professional SDK Call with ThinkingLevel Enum
        response = client.models.generate_content(
            model="gemini-3-flash-preview",
            contents=user_query,
            config=types.GenerateContentConfig(
                system_instruction=SYSTEM_PROMPT,
                response_mime_type="application/json",
                response_schema=response_schema,
                thinking_config=types.ThinkingConfig(
                    include_thoughts=True, 
                    thinking_level=types.ThinkingLevel.HIGH  # Max reasoning for UAF/Race detection
                )
            )
        )
        
        # SDK returns response.text as a clean JSON string ready for parsing
        return json.loads(response.text)

    except Exception as e:
        print(f"Audit processing error: {e}")
        return None

def main():
    patch_content = read_patch_file()
    if not patch_content:
        return

    print("--- [PHASE 2] Initiating Deep Reasoning Audit (Official SDK Mode) ---")
    analysis_data = generate_analysis(patch_content)

    if analysis_data:
        # Save structured results for HackerOne submission
        with open('analysis_results.json', 'w') as f:
            json.dump(analysis_data, f, indent=4)
        print("\n[SUCCESS] Kernel Security Audit Complete. Results saved.")
        
        # Display immediate summary
        for item in analysis_data:
            print(f"\nHash: {item.get('commit_hash')}")
            print(f"Security Level: {item.get('security_flag')}")
            print(f"Vulnerability Summary: {item.get('human_readable_summary')}")
    else:
        print("Critical Error: Audit failed to generate valid results.")

if __name__ == "__main__":
    main()
