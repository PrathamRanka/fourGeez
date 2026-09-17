$pythonCandidates = Get-ChildItem `
    -Path "$env:LOCALAPPDATA\Programs\Python\Python*\python.exe" `
    -ErrorAction SilentlyContinue |
    Sort-Object FullName -Descending

if (-not $pythonCandidates) {
    Write-Error "Python 3.11 or newer is required to run verification tests."
    exit 1
}

$pythonExecutable = $pythonCandidates[0].FullName
& $pythonExecutable `
    -m unittest discover `
    -s verification/python/tests `
    -t verification/python `
    -v
exit $LASTEXITCODE
