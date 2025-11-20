const express = require("express");
const { processMRWebhook } = require("./webhook/handler");

const app = express();
app.use(express.json());

// Simple logging
app.use((req, res, next) => {
  console.log(`[${new Date().toISOString()}] ${req.method} ${req.url}`);
  next();
});

app.post("/webhook/gitlab-mr", (req, res) => {
  // Respond immediately to GitLab to prevent timeout
  res.json({ status: "ok", message: "Processing started" });
  
  // Run async logic
  processMRWebhook(req.body).catch(err => {
    console.error("Unhandled Async Error:", err);
  });
});

module.exports = app;