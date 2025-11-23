import json
import os
import time
from dotenv import load_dotenv
import requests

# --- Configuration ---
load_dotenv()
# The API key must be provided either in a .env file or manually here.
API_KEY = os.getenv("GEMINI_API_KEY", "") 
# Use the correct model for structured output
API_URL = f"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-preview-09-2025:generateContent?key={API_KEY}"

# --- LLM System Instructions ---
SYSTEM_PROMPT = """
You are an expert Linux Kernel Engineer acting as an automated patch reviewer. 
Your primary goal is to analyze the provided code diff (patch) and commit message.
You must provide your analysis in a structured JSON format, following the provided schema.
The analysis must be concise, highlighting the security implications and potential for regression/conflicts.
"""

def read_patch_file(filepath="patches_to_analyze.txt"):
    """Reads the raw content of the Git log file."""
    try:
        # Use 'r' mode to read the text file
        with open(filepath, 'r', encoding='utf-8') as f:
            return f.read()
    except FileNotFoundError:
        print(f"Error: {filepath} not found. Ensure you ran the git log command correctly.")
        return None

def generate_analysis(patch_content):
    """Calls the Gemini API to get structured patch analysis."""
    
    # Define the desired JSON output structure (Structured Response)
    response_schema = {
        "type": "ARRAY",
        "items": {
            "type": "OBJECT",
            "properties": {
                "commit_hash": {"type": "STRING", "description": "The unique hash of the commit."},
                "title": {"type": "STRING", "description": "A concise, 1-sentence summary of the patch's main purpose."},
                "affected_subsystem": {"type": "STRING", "description": "The kernel subsystem most affected (e.g., 'Networking', 'Storage', 'Core')."},
                "security_flag": {"type": "STRING", "description": "HIGH, MEDIUM, LOW, or NONE based on potential security implications."},
                "potential_issues": {"type": "ARRAY", "items": {"type": "STRING"}, "description": "List any potential regressions, conflicts, or complexity issues."},
                "human_readable_summary": {"type": "STRING", "description": "A simple paragraph explaining the patch's technical impact."}
            }
        }
    }

    user_query = f"Analyze the following Linux Kernel patch data and return a JSON array containing structured analysis for each patch:\n\n--- PATCH DATA ---\n{patch_content}"
    
    payload = {
        "contents": [{"parts": [{"text": user_query}]}],
        "systemInstruction": {"parts": [{"text": SYSTEM_PROMPT}]},
        "generationConfig": {
            "responseMimeType": "application/json",
            "responseSchema": response_schema
        }
    }

    if not API_KEY:
        print("Error: API Key is missing. Please create a .env file with GEMINI_API_KEY='YOUR_KEY'.")
        return None

    # Implement Exponential Backoff for retries
    max_retries = 5
    delay = 1
    
    for i in range(max_retries):
        try:
            response = requests.post(
                API_URL, 
                headers={'Content-Type': 'application/json'}, 
                data=json.dumps(payload)
            )
            response.raise_for_status() # Raise an exception for HTTP errors (4xx or 5xx)
            
            result = response.json()
            
            # Extract the JSON string from the response
            json_text = result.get('candidates', [{}])[0].get('content', {}).get('parts', [{}])[0].get('text')
            
            if json_text:
                return json.loads(json_text)
            
        except requests.exceptions.RequestException:
            if i < max_retries - 1:
                time.sleep(delay)
                delay *= 2
            else:
                print(f"Failed to generate content after {max_retries} attempts.")
                return None
        except Exception as e:
             print(f"An unexpected error occurred: {e}")
             return None

    return None

def main():
    patch_content = read_patch_file()
    if not patch_content:
        return

    print("--- Sending patch data to AI for analysis... ---")
    analysis_data = generate_analysis(patch_content)
    
    if analysis_data:
        # Save the raw JSON output for submission proof
        with open('analysis_results.json', 'w') as f:
            json.dump(analysis_data, f, indent=4)
        print("\n[SUCCESS] Analysis saved to 'analysis_results.json'.")
        
        # Print a human-readable summary
        print("\n--- AI-Driven Patch Review Summary ---")
        for item in analysis_data:
            security_flag = item.get('security_flag', 'N/A')
            # Use ANSI color codes for visual emphasis on security flag
            color = '\033[92m' if security_flag == 'NONE' else ('\033[93m' if security_flag == 'MEDIUM' else '\033[91m')
            reset = '\033[0m'
            
            print(f"\nHash: {item.get('commit_hash', 'N/A')}")
            print(f"Title: {item.get('title', 'N/A')}")
            print(f"Subsystem: {item.get('affected_subsystem', 'N/A')}")
            print(f"Security Flag: {color}{security_flag}{reset}") 
            print(f"Issues: {', '.join(item.get('potential_issues', ['None']))}")
            print(f"Summary: {item.get('human_readable_summary', 'N/A')}")
            print("-" * 50)
        
        print("\n[Project Integration]: The structured JSON output is designed to feed into platforms like n8n for automated alerting and ticketing.")
    else:
        print("Analysis failed.")

if __name__ == "__main__":
    main()

