from mcp.server.fastmcp import FastMCP
import ffmpeg

# Create an MCP server
mcp = FastMCP("WhatsApp Bulk Sender v2.0")

@mcp.tool()
def get_unread_messages() -> str:
    """Fetch unread WhatsApp messages from the Go bridge."""
    return "No unread messages."

@mcp.tool()
def process_voice_note(filepath: str) -> str:
    """Process a voice note using FFmpeg."""
    # Example using ffmpeg-python
    # ffmpeg.input(filepath).output('output.wav').run()
    return f"Voice note {filepath} processed."

if __name__ == "__main__":
    # Initialize and run the server
    mcp.run()
