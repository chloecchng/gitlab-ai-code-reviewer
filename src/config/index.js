require("dotenv").config();

module.exports = {
  PORT: process.env.PORT || 5678,
  GITLAB_TOKEN: process.env.GITLAB_TOKEN,
  GEMINI_API_KEY: process.env.GEMINI_API_KEY,
  GITLAB_URL: process.env.GITLAB_URL,
  GEMINI_MODEL: "gemini-2.0-flash", 
};