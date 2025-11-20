const axios = require("axios");
const { GEMINI_API_KEY, GEMINI_MODEL } = require("../config");

/**
 * PRE-PROCESSOR:
 * Parses Git Diffs and adds EXPLICIT line numbers to the string.
 */
function addLineNumbersToDiff(diff) {
  const lines = diff.split(/\r?\n/);
  
  let currentLineNumber = 0;
  let processedLines = [];

  for (const line of lines) {
    // Guard: Skip empty strings that result from the last newline split
    if (line.length === 0) continue;

    // Parse the Chunk Header to find the starting line number
    // Format: @@ -oldStart,oldLen +newStart,newLen @@
    if (line.startsWith('@@')) {
      // Extract the NEW file starting line number (after the +)
      const match = line.match(/\+(\d+)/); 
      if (match) {
        currentLineNumber = parseInt(match[1], 10);
      }
      processedLines.push(line); // Keep header for context
      continue;
    }

    // Handle Content Lines
    if (line.startsWith('+')) {
      // New/Added line: Counts towards the new file line numbers
      processedLines.push(`${currentLineNumber}| ${line}`);
      currentLineNumber++;
    } else if (line.startsWith(' ')) {
      // Context line: Unchanged, but still counts towards line numbers
      processedLines.push(`${currentLineNumber}| ${line}`);
      currentLineNumber++;
    } else if (line.startsWith('-')) {
      // Deleted line: Only exists in OLD file. 
      // We mark it REM (Removed) so AI ignores it for commenting on new file.
      processedLines.push(`REM| ${line}`);
    } else {
      // Metadata like "\ No newline at end of file"
      processedLines.push(line);
    }
  }

  return processedLines.join('\n');
}

/**
 * Truncate very large files to prevent Token Exhaustion
 */
function truncateDiff(diff) {
  const MAX_CHARS = 15000; 
  if (!diff) return "";
  if (diff.length <= MAX_CHARS) return diff;
  return diff.substring(0, MAX_CHARS) + "\n... [Diff Truncated by AI System] ...";
}

async function analyzeWithGemini(diffChanges) {
  if (!diffChanges || diffChanges.length === 0) return [];

  // First, add explicit line numbers to the diffs
  // Then, truncate if too large
  const cleanedDiffs = diffChanges.map(d => ({
    file: d.new_path || d.old_path,
    diff: truncateDiff(addLineNumbersToDiff(d.diff)),
  }));

  const prompt = `
  ROLE: Senior Software Architect & Security Engineer.
  TASK: Review the following code changes for a GitLab Merge Request.
  
  RULES:
  1. Focus on: Logic Errors, Security Vulnerabilities, Crash Risks, and Performance bottlenecks.
  2. IGNORE: Formatting, indentation, simple naming preferences, or missing comments.
  3. Be constructive and provide code solutions.
  
  INPUT DATA (Diffs with explicit line numbers added as "LineNumber| Code"):
  ${JSON.stringify(cleanedDiffs)}

  OUTPUT FORMAT:
  Return a valid JSON ARRAY ONLY. No markdown, no code blocks.
  structure:
  [
    { 
      "file": "src/app.js", 
      "line": 14, 
      "comment": "Critical: This variable can be null. Use optional chaining." 
    }
  ]

  IMPORTANT INSTRUCTIONS FOR LINE NUMBERS:
  - The input diffs have explicit line numbers added at the start of valid lines.
  - Example input: "55| + const x = 1;" -> This is line 55.
  - You MUST use the integer provided before the pipe "|" symbol as the "line" value.
  - Do NOT count lines yourself. TRUST the numbers provided in the text.
  - Do not comment on lines marked "REM|" (Deleted lines).
  `;

  try {
    const response = await axios.post(
      `https://generativelanguage.googleapis.com/v1beta/models/${GEMINI_MODEL}:generateContent`,
      {
        contents: [{ role: "user", parts: [{ text: prompt }] }],
        generationConfig: { responseMimeType: "application/json" }
      },
      {
        headers: { "x-goog-api-key": GEMINI_API_KEY }
      }
    );

    const candidate = response.data.candidates[0].content.parts[0].text;
    const cleanJson = candidate.replace(/```json|```/g, "").trim();
    
    return JSON.parse(cleanJson);

  } catch (err) {
    console.error("❌ Gemini AI Error:", err.response?.data || err.message);
    return []; 
  }
}

module.exports = { analyzeWithGemini };