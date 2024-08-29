# ProjectAIli

ProjectAIli is an advanced command-line interface (CLI) application that enables users to interact with a powerful AI assistant powered by the llama3-70b-8192 model through the Groq API. This application provides a seamless and interactive chat experience with features like conversation saving, loading, and streaming responses.

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

## Table of Contents

- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage](#usage)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Error Handling](#error-handling)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)
- [Security](#security)
- [Performance Considerations](#performance-considerations)
- [Roadmap](#roadmap)

## Features

- Real-time chat with an AI assistant
- Streaming AI responses for a more natural conversation flow
- Save and load conversation history
- Dynamic conversation management to stay within token limits
- Graceful error handling and retry mechanism
- Rate limiting to comply with API restrictions
- Colorized output for improved readability

## Prerequisites

- Go 1.23.0 or higher
- Groq API key

## Installation

1. Clone the repository:
   ```
   git clone https://github.com/Milou4Dev/ProjectAIli.git
   cd ProjectAIli
   ```

2. Install dependencies:
   ```
   go mod download
   ```

3. Create a `config.yaml` file in the project root with your Groq API key:
   ```yaml
   groq_api_key: "your_groq_api_key_here"
   ```

## Usage

To start ProjectAIli, run:

```
go run main.go
```

Once the application is running, you can interact with the AI assistant by typing your messages. The AI will respond in real-time with streaming output.

### Commands

- Type your message and press Enter to send it to the AI assistant.
- Type `exit` to quit the application.
- Use `/save` to save the current conversation.
- Use `/load <filename>` to load a previously saved conversation.

## Getting Started

Here's a quick example of how to use ProjectAIli:

1. Start the application:
   ```
   go run main.go
   ```

2. Once the application is running, you'll see a prompt. Type your message and press Enter:
   ```
   You: Hello, AI assistant!
   AI: Hello! It's great to meet you. How can I assist you today?
   
   You: Can you explain what ProjectAIli is?
   AI: Certainly! ProjectAIli is an advanced command-line interface (CLI) application that allows users to interact with a powerful AI assistant, which is me. I'm powered by the llama3-70b-8192 model and communicate through the Groq API.

   ProjectAIli offers several key features:
   1. Real-time chat: You can have a natural conversation with me, just like we're doing now.
   2. Streaming responses: My responses are streamed in real-time, creating a more fluid interaction.
   3. Conversation management: You can save and load conversation histories, allowing you to continue discussions later.
   4. Token limit management: The application dynamically manages the conversation to stay within API token limits.
   5. Error handling and retries: It has robust error handling to ensure a smooth experience.
   6. Rate limiting: The app complies with API restrictions to prevent overuse.
   7. Colorized output: Responses are color-coded for improved readability.

   Is there anything specific about ProjectAIli you'd like to know more about?

   You: That's great, thank you!
   AI: You're welcome! I'm glad I could help explain ProjectAIli to you. It's a powerful tool designed to make AI interactions accessible and user-friendly. If you have any more questions about its features, usage, or capabilities, feel free to ask. Enjoy exploring and using ProjectAIli!

   You: exit
   AI: Goodbye! Thank you for using ProjectAIli. Have a great day!
   ```

## Configuration

ProjectAIli uses the following configuration files:

- `config.yaml`: Contains the Groq API key.
- `system_prompt.txt`: Defines the initial system prompt for the AI assistant.

## Architecture

The main application logic is contained in `main.go`. Key components include:

- API Client: Handles communication with the Groq API
- Conversation: Manages the chat history and token count
- Chat Loop: Processes user input and AI responses
- Request Handling: Prepares and sends requests to the API
- Response Processing: Handles streaming responses from the API
- Conversation Management: Implements save and load functionality

## Error Handling

ProjectAIli implements a robust error handling mechanism with retries for API requests. It also gracefully handles user interrupts and timeouts.

## Testing

To run the tests for ProjectAIli, use the following command:

```
go test ./...
```

This will run all tests in the project. Make sure to write tests for any new features or bug fixes you implement.

## Troubleshooting

Common issues and their solutions:

1. API Key Issues:
   - Ensure your Groq API key is correctly set in the `config.yaml` file.
   - Check that the `config.yaml` file is in the project root directory.

2. Rate Limiting:
   - If you encounter rate limiting errors, wait a few minutes before trying again.
   - Consider upgrading your API plan if you frequently hit rate limits.

3. Connection Issues:
   - Verify your internet connection is stable.
   - Check if the Groq API is experiencing any outages.

4. Unexpected Behavior:
   - Make sure you're using the latest version of ProjectAIli.
   - Clear your conversation history and start a new chat.

If you encounter any other issues, please open an issue on the GitHub repository.

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository
2. Create a new branch: `git checkout -b feature-branch-name`
3. Make your changes and commit them: `git commit -m 'Add some feature'`
4. Push to the branch: `git push origin feature-branch-name`
5. Submit a pull request

Please ensure your code adheres to the project's coding standards and includes appropriate tests.

## License

ProjectAIli is licensed under the GNU General Public License v3.0 (GNU GPLv3). This means:

- You are free to use, modify, and distribute this software.
- If you distribute modified versions, you must also distribute them under the GNU GPLv3.
- Any software that uses ProjectAIli must also be released under the GNU GPLv3.

For more details, please see the [GNU GPLv3 license text](https://www.gnu.org/licenses/gpl-3.0.en.html).

## Security

- ProjectAIli uses HTTPS for API communication.
- API keys are stored in a separate configuration file (`config.yaml`) which should be kept secure and not committed to version control.
- User inputs are validated and sanitized before processing.
- Regular security audits are performed on the codebase.
- We follow best practices for secure coding and keep dependencies up to date.

## Performance Considerations

- The application uses streaming responses for real-time interaction.
- Conversation history is dynamically managed to stay within token limits.
- Rate limiting is implemented to comply with API restrictions.
- Efficient data structures are used to minimize memory usage.
- The application is designed to handle long-running conversations without degradation in performance.

For optimal performance, ensure a stable internet connection and sufficient system resources.

## Roadmap

Future plans for ProjectAIli include:

- [ ] Implement multi-user support
- [ ] Add support for multiple AI models
- [ ] Implement conversation summarization feature
- [ ] Add support for voice input and output
