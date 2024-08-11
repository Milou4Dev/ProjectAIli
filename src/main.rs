use anyhow::{Context, Result};
use console::{style, Term};
use dialoguer::{theme::ColorfulTheme, Input};
use futures_util::StreamExt;
use indicatif::{ProgressBar, ProgressStyle};
use once_cell::sync::Lazy;
use reqwest::{Client, Response};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::{
    fs,
    io::{self, Write},
    sync::Arc,
    time::Duration,
};
use tiktoken_rs::cl100k_base;
use tokio::sync::RwLock;

const MAX_TOKENS: usize = 8000;
const API_URL: &str = "https://api.groq.com/openai/v1/chat/completions";
const INITIAL_HISTORY_CAPACITY: usize = 10;
const CONFIG_FILE: &str = "config.yaml";
const TIMEOUT_SECONDS: u64 = 30;
const SPINNER_INTERVAL_MS: u64 = 100;

static TOKENIZER: Lazy<tiktoken_rs::CoreBPE> =
    Lazy::new(|| cl100k_base().expect("Failed to load tokenizer"));

#[derive(Deserialize, Clone)]
struct Config {
    groq_api_key: String,
}

#[derive(Default, Clone, Serialize, Deserialize)]
struct Message {
    role: String,
    content: String,
}

struct Conversation {
    history: Vec<Message>,
}

#[tokio::main]
async fn main() -> Result<()> {
    let config = Arc::new(load_config().context("Failed to load configuration")?);
    let client = Arc::new(create_client()?);
    let conversation = Arc::new(RwLock::new(Conversation::new()));

    print_welcome_message();

    while let Some(user_input) = get_user_input().await? {
        conversation.write().await.add_message("user", &user_input);

        let spinner = create_spinner();
        let response = send_api_request(&client, &config, &conversation).await?;
        spinner.finish_and_clear();

        print!("{}", style("AI: ").magenta().bold());
        io::stdout().flush()?;

        let ai_response = process_stream_response(response).await?;

        conversation
            .write()
            .await
            .add_message("assistant", &ai_response);

        println!();
    }

    Ok(())
}

fn load_config() -> Result<Config> {
    fs::read_to_string(CONFIG_FILE)
        .context("Failed to read config file")
        .and_then(|contents| serde_yaml::from_str(&contents).context("Failed to parse config file"))
}

fn create_client() -> Result<Client> {
    Client::builder()
        .timeout(Duration::from_secs(TIMEOUT_SECONDS))
        .build()
        .context("Failed to create HTTP client")
}

fn print_welcome_message() {
    let term = Term::stdout();
    term.clear_screen().expect("Failed to clear screen");
    println!(
        "{}",
        style("Welcome to the AI Chat!").cyan().bold().underlined()
    );
    println!(
        "{}",
        style("Type 'exit' to quit the program.").blue().italic()
    );
    println!();
}

async fn get_user_input() -> Result<Option<String>> {
    let user_input: String = Input::with_theme(&ColorfulTheme::default())
        .with_prompt(style("You").green().bold().to_string())
        .allow_empty(true)
        .interact_text()
        .context("Failed to get user input")?;

    if user_input.trim().eq_ignore_ascii_case("exit") {
        println!("{}", style("Goodbye!").yellow().bold());
        Ok(None)
    } else {
        Ok(Some(user_input))
    }
}

impl Conversation {
    fn new() -> Self {
        Self {
            history: Vec::with_capacity(INITIAL_HISTORY_CAPACITY),
        }
    }

    fn add_message(&mut self, role: &str, content: &str) {
        self.history.push(Message {
            role: role.to_string(),
            content: content.to_string(),
        });
    }

    fn history(&self) -> &[Message] {
        &self.history
    }
}

async fn send_api_request(
    client: &Client,
    config: &Config,
    conversation: &RwLock<Conversation>,
) -> Result<Response> {
    let conv = conversation.read().await;
    let conversation_history = conv.history();
    let total_tokens = count_tokens(conversation_history);

    let truncated_history = if total_tokens > MAX_TOKENS {
        truncate_conversation(conversation_history, MAX_TOKENS)
    } else {
        conversation_history.to_vec()
    };
    drop(conv);

    client
        .post(API_URL)
        .header("Content-Type", "application/json")
        .header("Authorization", format!("Bearer {}", config.groq_api_key))
        .json(&create_request_body(&truncated_history))
        .send()
        .await
        .context("Failed to send request")
}

fn count_tokens(conversation_history: &[Message]) -> usize {
    conversation_history
        .iter()
        .map(|message| TOKENIZER.encode_ordinary(&message.content).len())
        .sum()
}

fn truncate_conversation(conversation_history: &[Message], max_tokens: usize) -> Vec<Message> {
    let mut truncated = Vec::new();
    let mut total_tokens = 0;

    for message in conversation_history.iter().rev() {
        let tokens = TOKENIZER.encode_ordinary(&message.content).len();
        if total_tokens + tokens > max_tokens {
            break;
        }
        total_tokens += tokens;
        truncated.push(message.clone());
    }

    truncated.reverse();
    truncated
}

fn create_request_body(truncated_history: &[Message]) -> Value {
    serde_json::json!({
        "messages": truncated_history,
        "model": "llama3-70b-8192",
        "temperature": 0.7,
        "max_tokens": MAX_TOKENS,
        "top_p": 0.9,
        "stream": true,
        "stop": null
    })
}

async fn process_stream_response(response: Response) -> Result<String> {
    let mut stream = response.bytes_stream();
    let mut buffer = String::with_capacity(1024);

    while let Some(item) = stream.next().await {
        let chunk = item.context("Failed to read stream chunk")?;
        let chunk_str = String::from_utf8_lossy(&chunk);

        for line in chunk_str.lines() {
            if let Some(data) = line.strip_prefix("data: ") {
                if data == "[DONE]" {
                    return Ok(buffer.trim().to_string());
                }
                if let Ok(json) = serde_json::from_str::<Value>(data) {
                    if let Some(content) = json["choices"][0]["delta"]["content"].as_str() {
                        print!("{}", style(content).white());
                        io::stdout().flush()?;
                        buffer.push_str(content);
                    }
                }
            }
        }
    }

    Ok(buffer.trim().to_string())
}

fn create_spinner() -> ProgressBar {
    let spinner = ProgressBar::new_spinner();
    spinner.set_message("AI is thinking...");
    spinner.set_style(
        ProgressStyle::default_spinner()
            .tick_chars("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
            .template("{spinner} {msg}")
            .expect("Failed to set spinner style"),
    );
    spinner.enable_steady_tick(Duration::from_millis(SPINNER_INTERVAL_MS));
    spinner
}
