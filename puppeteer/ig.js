const puppeteer = require('puppeteer');
const fs = require('fs');

(async () => {
  // Launch a new browser instance
  const browser = await puppeteer.launch();

  // Open a new page
  const page = await browser.newPage();
  await page.setRequestInterception(true);
  // Create a log file to store HTTP requests and responses
  const logFilePath = 'http_logs.txt';
  const logStream = fs.createWriteStream(logFilePath, { flags: 'a' });

  // Listen for requests
  page.on('request', (request) => {
    logStream.write(`REQUEST: ${request.method()} ${request.url()}\n`);
  });

  // Listen for responses
  page.on('response', async (response) => {
    logStream.write(`RESPONSE: ${response.status()} ${response.url()}\n`);

    // Check if the response is a redirect
    if (response.status() >= 300 && response.status() < 400) {
      logStream.write(`REDIRECTED\n\n`);
    } else {
      // Read and log the response body
      const responseBody = await response.text();
      logStream.write(`BODY:\n${responseBody}\n\n`);
    }
  });

  // Navigate to a website
  await page.goto('https://instagram.com/maroon5/');

  // Take a screenshot
  await page.screenshot({ path: 'example.png' });

  // Close the browser
  await browser.close();

  // Close the log stream
  logStream.end();
})();
