// src/config/index.js
require("dotenv").config();

module.exports = {
  // Use CI_SERVER_URL if GITLAB_URL is not explicitly provided
  GITLAB_URL: process.env.GITLAB_URL || process.env.CI_SERVER_URL || "https://gitlab.com",
  GITLAB_TOKEN: process.env.GITLAB_TOKEN,
  GEMINI_API_KEY: process.env.GEMINI_API_KEY,
  GEMINI_MODEL: "gemini-2.0-flash", 
};