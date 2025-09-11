# Contributing to Fansly Scraper

First off, thank you for considering contributing! We love to receive contributions from the community, whether it's reporting a bug, discussing a new feature, or writing code.

This document provides a set of guidelines for contributing to the project.

## How Can I Contribute?

### Reporting Bugs
If you find a bug, please ensure it hasn't already been reported by searching through the [GitHub Issues](https://github.com/agnosto/fansly-scraper/issues).

If you're unable to find an open issue addressing the problem, [open a new one](https://github.com/agnosto/fansly-scraper/issues/new?template=bug_report.md). Be sure to include a **title and clear description**, as much relevant information as possible, and a **code sample or an executable test case** demonstrating the expected behavior that is not occurring.

### Suggesting Enhancements
If you have an idea for a new feature, please [open a new feature request](https://github.com/agnosto/fansly-scraper/issues/new?template=feature_request.md). A clear description of the feature and the problem it solves will help get the conversation started.

### Pull Requests
We welcome pull requests! If you're planning to implement a new feature, it's a good idea to discuss it in a feature request issue first to ensure it aligns with the project's goals.

## Development Setup

To get started with the code, follow these steps:

1.  **Prerequisites:**
    *   Go (version 1.23 or newer is recommended)
    *   Git

2.  **Fork and Clone:**
    *   Fork the repository on GitHub.
    *   Clone your forked repository to your local machine:
        ```bash
        git clone https://github.com/your-username/fansly-scraper.git
        cd fansly-scraper
        ```

3.  **Build the Project:**
    *   You can build the project using the standard Go build command. For a production-like build, use the following:
        ```bash
        go build -v -ldflags "-w -s" -o fansly-scraper ./cmd/fansly-scraper
        ```

## Pull Request Process

1.  Create a new branch for your changes:
    ```bash
    git checkout -b feature/my-awesome-feature
    ```
2.  Make your changes to the code.

3.  **Test Your Changes Thoroughly**
    Since we do not currently have an automated test suite, manual testing is a critical part of the contribution process. Before submitting a pull request, please ensure your changes do not break existing functionality.
    
    -   **If you modified downloading logic:**
        -   Verify that content downloads successfully from different areas (Timeline, Messages, Stories, Purchases).
        -   Confirm that the highest available quality is being saved.
        -   Ensure that posts with multiple media items (e.g., an album with 7 images and 3 videos) are downloaded completely.
    
    -   **If you modified the TUI (Terminal User Interface):**
        -   Ensure all menus are still navigable without errors.
        -   Confirm that UI elements (progress bars, status messages, lists) update correctly and do not have rendering glitches.
        -   Verify that user flows are not broken (e.g., selecting a model, downloading, and returning to the main menu).
    
    -   **For any change:**
        -   Run the application and test core features to ensure your changes have not introduced any regressions in unrelated areas.

4.  **Format your code.** Before committing, please run the following command from the root of the repository to ensure your code is formatted correctly. The `...` ensures it formats all subdirectories.
    ```bash
    go fmt ./...
    ```
5.  Commit your changes with a descriptive commit message. We follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) standard, which helps automate our release notes.
    ```bash
    git commit -m "feat: Add support for downloading albums"
    ```
6.  Push your branch to your fork on GitHub:
    ```bash
    git push origin feature/my-awesome-feature
    ```
7.  Open a pull request from your forked repository to the `main` branch of the original repository. Please provide a clear description of the changes in the pull request and summarize the manual testing you performed.

Thank you for your contribution!
