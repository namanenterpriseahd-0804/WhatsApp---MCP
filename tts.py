from gtts import gTTS
import os

text = "This is an Google Antigravity (Test Message)"
tts = gTTS(text, lang='en')
tts.save("voice.mp3")
print("Saved voice.mp3")
