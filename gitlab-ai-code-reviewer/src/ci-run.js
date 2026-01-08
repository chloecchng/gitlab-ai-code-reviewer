// src/ci-run.js
const { getMRDetails } = require("./gitlab/api");
const { analyzeWithGemini } = require("./ai/gemini");
const { postInlineComment } = require("./gitlab/comments");

async function startCiReview() {
  console.log("🤖 Starting AI Code Review via GitLab CI...");

  if (!process.env.GITLAB_TOKEN) {
    console.error("❌ GITLAB_TOKEN is missing from the environment!");
  } else {
    console.log("✅ GITLAB_TOKEN is detected (length: " + process.env.GITLAB_TOKEN.length + ")");
  }
  
  const projectId = process.env.CI_PROJECT_ID;
  const mrIid = process.env.CI_MERGE_REQUEST_IID;

  if (!mrIid) {
    console.error("❌ Error: This job must run in a Merge Request context.");
    process.exit(1);
  }

  try {
    // 1. Fetch the actual code changes from the GitLab API
    console.log(`Fetching changes for MR !${mrIid}...`);
    const mrData = await getMRDetails(projectId, mrIid);
    const diffChanges = mrData.changes;

    if (!diffChanges || diffChanges.length === 0) {
      console.log("✅ No reviewable code changes found.");
      process.exit(0);
    }

    // 2. Run AI Analysis
    console.log(`🔍 Analyzing ${diffChanges.length} files...`);
    const reviews = await analyzeWithGemini(diffChanges);
    console.log(`🤖 AI generated ${reviews.length} suggestions.`);

    // 3. Post comments back to GitLab
    for (const review of reviews) {
      await postInlineComment(projectId, mrIid, review, mrData);
      // Brief delay to avoid hitting GitLab API rate limits
      await new Promise(resolve => setTimeout(resolve, 500));
    }

    console.log("✅ Review completed successfully.");
    process.exit(0);
  } catch (error) {
    console.error("💥 Review failed:", error.message);
    process.exit(1);
  }
}

startCiReview();