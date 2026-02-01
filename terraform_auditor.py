import json
import os
from dotenv import load_dotenv
# Official Google SDK for 2026
from google import genai
from google.genai import types

load_dotenv()
API_KEY = os.getenv("GEMINI_API_KEY", "")

# Initialize the official client
client = genai.Client(api_key=API_KEY)

def generate_audit_payload(terraform_code: str):
    """Performs deep-reasoning IaC audit using the Gemini 3 SDK."""
    
    system_prompt = (
        "You are a DevSecOps Expert. Analyze this Terraform HCL for security risks. "
        "Trace network paths and IAM permissions. Output ONLY a valid JSON array."
    )

    # Define the strict structure for your DevSecOps pipeline
    audit_schema = {
        "type": "ARRAY",
        "items": {
            "type": "OBJECT",
            "properties": {
                "resource_type": {"type": "STRING"},
                "resource_name": {"type": "STRING"},
                "finding": {"type": "STRING"},
                "severity": {"type": "STRING"},
                "remediation": {"type": "STRING"}
            }
        }
    }

    # Using the SDK method for 'High' Thinking reasoning
    response = client.models.generate_content(
        model="gemini-3-flash-preview",
        contents=f"Audit this Terraform code:\n{terraform_code}",
        config=types.GenerateContentConfig(
            system_instruction=system_prompt,
            response_mime_type="application/json",
            # This enables the JSON schema enforcement
            response_schema=audit_schema,
            thinking_config={
                "include_thoughts": True, 
                "thinking_level": "HIGH"
            }
        )
    )

    return json.loads(response.text)

def main():
    try:
        # Load your test infrastructure file
        with open('test_infra.tf', 'r') as f:
            code = f.read()
        
        print("--- [PHASE 2] Starting SDK-Based Terraform Audit... ---")
        findings = generate_audit_payload(code)
        
        # Save results for your portfolio
        with open('terraform_audit_results.json', 'w') as f:
            json.dump(findings, f, indent=4)
            
        print(f"[SUCCESS] {len(findings)} issues identified and logged.")
    except Exception as e:
        print(f"Audit Error: {e}")

if __name__ == "__main__":
    main()
