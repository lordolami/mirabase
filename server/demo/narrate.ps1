param([Parameter(Mandatory=$true)][string]$StoryFile,[Parameter(Mandatory=$true)][string]$OutputDirectory)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Speech
[IO.Directory]::CreateDirectory([IO.Path]::GetFullPath($OutputDirectory)) | Out-Null
$speaker=New-Object System.Speech.Synthesis.SpeechSynthesizer
try {
 $speaker.SelectVoice('Microsoft Zira Desktop')
 $speaker.Rate=1
 foreach($scene in (Get-Content -LiteralPath $StoryFile -Raw -Encoding utf8 | ConvertFrom-Json)) {
  $speaker.SetOutputToWaveFile((Join-Path $OutputDirectory ($scene.id+'.wav')))
  $speaker.Speak([string]$scene.text)
  $speaker.SetOutputToNull()
 }
} finally {$speaker.Dispose()}
