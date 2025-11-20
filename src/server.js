const app = require("./app");
const { PORT } = require("./config");

app.listen(PORT, () => {
  console.log(`🚀 GitBot Server running on port ${PORT}`);
});