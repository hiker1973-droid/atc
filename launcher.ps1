# vSFG-7 role launcher — small WinForms panel for the run_*.bat fleet.
# Auto-discovers every run_*.bat in this directory, reads the window title
# from the 'start "Title"' line, and gives you start/stop per role.

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$root  = Split-Path -Parent $MyInvocation.MyCommand.Path
$bats  = Get-ChildItem -Path $root -Filter 'run_*.bat'
$roles = @()
foreach ($b in $bats) {
    $title = $b.BaseName
    $content = Get-Content $b.FullName -Raw
    if ($content -match 'start\s+"([^"]+)"') { $title = $matches[1] }
    $roles += [PSCustomObject]@{ Name = $title; Bat = $b.FullName; File = $b.Name }
}

function Get-Status($title) {
    $p = Get-Process cmd -ErrorAction SilentlyContinue |
         Where-Object { $_.MainWindowTitle -eq $title }
    if ($p) { 'RUNNING' } else { 'stopped' }
}

function Start-Role($bat) {
    Start-Process -FilePath $bat -WorkingDirectory $root
}

function Stop-Role($title) {
    $cmds = Get-Process cmd -ErrorAction SilentlyContinue |
            Where-Object { $_.MainWindowTitle -eq $title }
    foreach ($p in $cmds) {
        Get-CimInstance Win32_Process -Filter "ParentProcessId=$($p.Id)" |
            ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
        Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
    }
}

$form = New-Object Windows.Forms.Form
$form.Text = 'vSFG-7 Launcher'
$form.Size = New-Object Drawing.Size(560, 380)
$form.StartPosition = 'CenterScreen'

$grid = New-Object Windows.Forms.DataGridView
$grid.Location = New-Object Drawing.Point(10, 10)
$grid.Size = New-Object Drawing.Size(520, 260)
$grid.AutoSizeColumnsMode = 'Fill'
$grid.RowHeadersVisible = $false
$grid.AllowUserToAddRows = $false
$grid.ReadOnly = $true
$grid.SelectionMode = 'FullRowSelect'
$grid.MultiSelect = $false
$null = $grid.Columns.Add('Name',   'Role')
$null = $grid.Columns.Add('Status', 'Status')
$null = $grid.Columns.Add('File',   'Bat file')
foreach ($r in $roles) { $null = $grid.Rows.Add($r.Name, (Get-Status $r.Name), $r.File) }
$form.Controls.Add($grid)

function Refresh-Grid {
    for ($i = 0; $i -lt $grid.Rows.Count; $i++) {
        $status = Get-Status $grid.Rows[$i].Cells['Name'].Value
        $grid.Rows[$i].Cells['Status'].Value = $status
        $color = if ($status -eq 'RUNNING') { [Drawing.Color]::FromArgb(220,255,220) } else { [Drawing.Color]::White }
        $grid.Rows[$i].DefaultCellStyle.BackColor = $color
    }
}

function Add-Button($text, $x, $handler) {
    $b = New-Object Windows.Forms.Button
    $b.Text = $text
    $b.Location = New-Object Drawing.Point($x, 285)
    $b.Size = New-Object Drawing.Size(100, 30)
    $b.Add_Click($handler)
    $form.Controls.Add($b)
}

Add-Button 'Start'     10  {
    $row = $grid.SelectedRows | Select-Object -First 1
    if ($row) {
        $bat = ($roles | Where-Object Name -eq $row.Cells['Name'].Value).Bat
        Start-Role $bat; Start-Sleep -Milliseconds 600; Refresh-Grid
    }
}
Add-Button 'Stop'      120 {
    $row = $grid.SelectedRows | Select-Object -First 1
    if ($row) { Stop-Role $row.Cells['Name'].Value; Start-Sleep -Milliseconds 400; Refresh-Grid }
}
Add-Button 'Start all' 230 {
    foreach ($r in $roles) {
        if ((Get-Status $r.Name) -ne 'RUNNING') { Start-Role $r.Bat; Start-Sleep -Seconds 1 }
    }
    Refresh-Grid
}
Add-Button 'Stop all'  340 {
    foreach ($r in $roles) { Stop-Role $r.Name }
    Start-Sleep -Milliseconds 500; Refresh-Grid
}
Add-Button 'Refresh'   450 { Refresh-Grid }

$timer = New-Object Windows.Forms.Timer
$timer.Interval = 3000
$timer.Add_Tick({ Refresh-Grid })
$timer.Start()
Refresh-Grid

[void]$form.ShowDialog()
