const form = document.querySelector("form");
const queryInput = document.getElementById("query");
const book_select = document.getElementById("book_select");
const resultsDiv = document.getElementById("results");
const statusDiv = document.getElementById("status");
const highlightEnabled = document.documentElement.dataset.highlight === "true";
const search_books = document.getElementById("search_books");

form.onsubmit = (event) => {
  event.preventDefault();

  const query = queryInput.value.trim();
  if (query == "") {
    localStorage.removeItem("query");
    resultsDiv.innerHTML = "";
    statusDiv.innerText = "Please enter a search query";
    return;
  }
  const book = book_select.value;

  const url = `/search?query=${query}&book=${book}`;

  try {
    handleSearch(url);
    localStorage.setItem("query", query);
    localStorage.setItem("book", book);
  } catch (error) {
    console.error(error);
    alert("An error occurred. Please try again.");
  }
};

async function handleSearch(url) {
  const controller = new AbortController();
  const signal = controller.signal;
  const timeout = 10000; // Timeout after 10 seconds

  const timeoutId = setTimeout(() => {
    controller.abort("Request timed out");
    statusDiv.innerText = "Request timed out. Please try again.";
  }, timeout);

  const start = performance.now();
  const res = await fetch(url, {
    signal,
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
  });

  // Clear the timeout to cancel the abort controller
  // and to avoid memory leaks.
  clearTimeout(timeoutId);

  if (!res.ok) {
    statusDiv.innerText = "An error occurred. Please try again.";
    return;
  }

  const data = await res.json();
  const end = performance.now();
  displayResults(data, start, end);
}

function displayResults(data, start, end) {
  // Clear the results.
  resultsDiv.innerHTML = "";
  statusDiv.innerHTML = "";

  data.forEach((match) => {
    // Create a wrapper div
    const result = document.createElement("div");
    result.className = "result";
    resultsDiv.appendChild(result);

    const anchor = document.createElement("a");
    anchor.href = `/books/${match.ID}/${match.PageNum}`;
    anchor.innerText = match.Text;
    anchor.target = "_blank";
    anchor.rel = "noopener noreferer";
    result.appendChild(anchor);

    // Add context
    const ctx = document.createElement("p");
    ctx.className = "snippet";
    if (highlightEnabled) {
      ctx.innerHTML = highlightText(
        match.Context,
        queryInput.value,
        "highlight"
      );
    } else {
      ctx.innerText = match.Context;
    }

    result.appendChild(ctx);

    // book title
    const book = document.createElement("p");
    book.className = "book";
    book.innerText = match.BaseName;
    result.appendChild(book);
  });

  const numberFormatter = new Intl.NumberFormat({
    style: "decimal",
    maximumFractionDigits: 0,
  });
  const num_results = numberFormatter.format(data.length);
  statusDiv.innerText = `Got ${num_results} results in ${(
    parseFloat(end - start) / 1000
  ).toFixed(1)}s`;
}

// Load the last query
const lastQuery = localStorage.getItem("query");
const lastBook = localStorage.getItem("book");
if (lastQuery) {
  queryInput.value = lastQuery;
  book_select.value = lastBook;

  handleSearch(`/search?query=${lastQuery}&book=${lastBook}`);
}

function highlightText(text, searchTerm, className) {
  const regex = new RegExp(searchTerm, "gi");
  const highlightedText = text.replace(
    regex,
    (match) => `<span class="${className}">${match}</span>`
  );

  return highlightedText;
}
