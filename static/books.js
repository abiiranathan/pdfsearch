const books = document.querySelectorAll(".book_link");

search_books.addEventListener("keyup", () => {
  const query = search_books.value.trim().toLowerCase();
  for (const book of books) {
    const text = book.innerText.trim().toLowerCase();
    if (text.indexOf(query) > -1) {
      book.classList.remove("hidden");
    } else {
      book.classList.add("hidden");
    }
  }
});
