import requests
import json
import os
import time
from dotenv import load_dotenv

# --- Configuration ---
load_dotenv()
API_KEY = os.getenv("GEMINI_API_KEY", "")
MODEL_NAME = "gemini-2.5-flash-preview-09-2025"
API_URL = f"https://generativelanguage.googleapis.com/v1beta/models/{MODEL_NAME}:generateContent?key={API_KEY}"
MAX_RETRIES = 5

def generate_audit_payload(terraform_code: str) -> dict:
    """Constructs the API payload for the Terraform security audit."""
    
    # The system instruction guides the LLM to act as a security expert
    system_prompt = (
        "You are a world-class DevSecOps security auditor specializing in Infrastructure as Code (IaC) "
        "and Terraform. Your task is to analyze the provided Terraform configuration, identify all security "
        "and compliance risks, and generate a structured JSON report. Focus on issues like "
        "public access, overly permissive IAM roles, unencrypted storage, and weak network ingress/egress rules."
    )

    # The user query provides the content and instructs the format
    user_query = (
        "Analyze the following Terraform configuration code for security vulnerabilities. "
        "Generate the audit findings in the exact JSON format specified in the schema. "
        "Code to analyze:\n\n---\n"
        f"{terraform_code}\n---"
    )

    # Define the strict JSON schema for the output report
    audit_schema = {
        "type": "ARRAY",
        "items": {
            "type": "OBJECT",
            "properties": {
                "resource_type": {"type": "STRING", "description": "e.g., aws_s3_bucket, google_compute_firewall"},
                "resource_name": {"type": "STRING", "description": "The name given to the resource in the HCL file."},
                "finding": {"type": "STRING", "description": "A concise description of the security issue found."},
                "severity": {"type": "STRING", "description": "The security risk level: HIGH, MEDIUM, LOW, or INFO."},
                "remediation": {"type": "STRING", "description": "A specific, actionable Terraform code suggestion to fix the issue."}
            },
            "propertyOrdering": ["resource_type", "resource_name", "finding", "severity", "remediation"]
        }
    }

    payload = {
        "contents": [{"parts": [{"text": user_query}]}],
        "systemInstruction": {"parts": [{"text": system_prompt}]},
        "generationConfig": {
            "responseMimeType": "application/json",
            "responseSchema": audit_schema
        }
    }
    return payload

def call_gemini_api(payload: dict) -> list:
    """Calls the Gemini API with exponential backoff for resilience."""
    for attempt in range(MAX_RETRIES):
        try:
            response = requests.post(
                API_URL, 
                headers={'Content-Type': 'application/json'}, 
                data=json.dumps(payload)
            )
            response.raise_for_status() # Raise HTTPError for bad responses (4xx or 5xx)

            result = response.json()
            
            # Check for content in the structured response
            if (result.get('candidates') and 
                result['candidates'][0].get('content') and 
                result['candidates'][0]['content'].get('parts')):
                
                json_text = result['candidates'][0]['content']['parts'][0]['text']
                
                # The model returns a string representation of the JSON array, so we parse it.
                return json.loads(json_text)
            
            print(f"Attempt {attempt + 1}: API response received but no valid content found.")
            time.sleep(2 ** attempt) # Exponential backoff
        
        except requests.exceptions.HTTPError as e:
            print(f"Attempt {attempt + 1}: HTTP error occurred: {e}. Retrying...")
            time.sleep(2 ** attempt)
        except Exception as e:
            print(f"Attempt {attempt + 1}: An unexpected error occurred: {e}. Retrying...")
            time.sleep(2 ** attempt)

    print("--- [FAILURE] Max retries reached. Could not complete API call. ---")
    return []

def print_audit_summary(findings: list):
    """Prints the security audit findings in a human-readable format."""
    print("\n--- AI-Driven Terraform Security Audit Summary ---")
    if not findings:
        print("[SUCCESS] No critical issues found or analysis failed.")
        return

    print(f"Total Findings: {len(findings)}\n")
    
    # Sort by severity for immediate visibility of critical issues
    severity_order = {"HIGH": 0, "MEDIUM": 1, "LOW": 2, "INFO": 3}
    sorted_findings = sorted(findings, key=lambda x: severity_order.get(x.get('severity', 'INFO'), 3))

    for i, finding in enumerate(sorted_findings):
        sev = finding.get('severity', 'INFO')
        print(f"[{i + 1}] SEVERITY: {sev}")
        print(f"  RESOURCE: {finding.get('resource_type', 'N/A')} ({finding.get('resource_name', 'N/A')})")
        print(f"  ISSUE:    {finding.get('finding', 'No description provided.')}")
        print(f"  FIX:      {finding.get('remediation', 'N/A')}")
        print("-" * 50)

def main():
    """Main function to run the Terraform auditor."""
    try:
        # 1. Read the Terraform code from the file
        tf_filepath = 'test_infra.tf'
        with open(tf_filepath, 'r') as f:
            terraform_code = f.read()
        
        print(f"--- Analyzing Terraform file: {tf_filepath} ({len(terraform_code)} bytes) ---")
        
        # 2. Generate the API payload
        payload = generate_audit_payload(terraform_code)
        
        # 3. Call the AI and get structured JSON findings
        findings = call_gemini_api(payload)
        
        # 4. Save the raw JSON output (for integration into other tools)
        output_filename = 'terraform_audit_results.json'
        with open(output_filename, 'w') as f:
            json.dump(findings, f, indent=2)
            
        print(f"\n[SUCCESS] Structured JSON audit saved to '{output_filename}'.")
        
        # 5. Print a human-readable summary
        print_audit_summary(findings)
        
    except FileNotFoundError:
        print(f"--- [ERROR] File not found: '{tf_filepath}'. Ensure 'test_infra.tf' exists. ---")
    except Exception as e:
        print(f"--- [CRITICAL ERROR] Failed to run auditor: {e} ---")

if __name__ == "__main__":
    main()
