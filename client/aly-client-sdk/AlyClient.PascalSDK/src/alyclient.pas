unit alyclient;

{$mode delphi}{$H+}

interface

uses
  SysUtils, Classes, Process, Windows,
  fpjson, jsonparser, models_gen, jsonhelp;

const
  DefaultShortTimeoutMs = 30000;
  DefaultRollbackTimeoutMs = 600000;
  DefaultDownloadStallTimeoutMs = 1800000;

  StatusNone = 'none';
  StatusDiscoveredUpdate = 'discoveredUpdate';
  StatusDownloadingUpdate = 'downloadingUpdate';
  StatusDownloadedUpdate = 'downloadedUpdate';
  StatusApplyUpdate = 'applyUpdate';

type
  TMyCriticalSection = class
  private
    FCS: TRTLCriticalSection;
  public
    constructor Create;
    destructor Destroy; override;
    procedure Enter;
    procedure Leave;
  end;

  TCancelToken = class
  private
    FLock: TMyCriticalSection;
    FCancelled: Boolean;
  public
    constructor Create;
    destructor Destroy; override;
    procedure Cancel;
    function IsCancelled: Boolean;
  end;

  TCloseOptions = record
    MustCloseProcessNames: string;
    CloseTimeoutSeconds: Integer;
  end;

  TCleanOptions = record
    KeepCount: Integer;
    All: Boolean;
  end;

  TDownloadProgressInfo = record
    FileName: string;
    Ratio: Double;
    Status: string;
    Index: Int64;
    Total: Int64;
    DownloadedBytes: Int64;
    TotalBytes: Int64;
    SpeedBps: Int64;
  end;

  TProgressCallback = procedure(const Info: TDownloadProgressInfo) of object;

  TDownloadOptions = record
    StallTimeoutMs: Integer;
    OnProgress: TProgressCallback;
    Cancel: TCancelToken;
  end;

  TBasicResponse = record
    IsSuccess: Boolean;
    ErrorMsg: string;
  end;

  TCheckUpdateResponse = record
    IsSuccess: Boolean;
    ErrorMsg: string;
    HasData: Boolean;
    Data: TCheckUpdateData;
  end;

  TCheckSelfUpdateResponse = record
    IsSuccess: Boolean;
    ErrorMsg: string;
    HasData: Boolean;
    Data: TCheckSelfUpdateData;
  end;

  TSelfUpdateResponse = record
    IsSuccess: Boolean;
    ErrorMsg: string;
    HasData: Boolean;
    Data: TSelfUpdateData;
  end;

  TRollbackListResponse = record
    IsSuccess: Boolean;
    ErrorMsg: string;
    HasData: Boolean;
    Data: TRollbackListData;
  end;

  TRollbackResponse = record
    IsSuccess: Boolean;
    ErrorMsg: string;
    HasData: Boolean;
    Data: TRollbackData;
  end;

  TCheckDiffResponse = record
    IsSuccess: Boolean;
    ErrorMsg: string;
    HasData: Boolean;
    Data: TCheckDiffData;
  end;

  TCleanResponse = record
    IsSuccess: Boolean;
    ErrorMsg: string;
    HasData: Boolean;
    Data: TCleanData;
  end;

  TApi = class
  private
    FPath: string;
    function StartProcess(const Args: array of string; Capture: Boolean): TProcess;
    function Execute(const Args: array of string; TimeoutMs: Integer; Cancel: TCancelToken;
      out StdOut, StdErr: string; out ExitCode: Integer; out TimedOut, Cancelled: Boolean;
      out StartError: string): Boolean;
    function RunCommand(const Args: array of string; TimeoutMs: Integer; Cancel: TCancelToken;
      out Root: TJSONObject; out ErrorMsg: string): Boolean;
  public
    constructor Create(const Path: string);
    destructor Destroy; override;
    function Path: string;
    class function DefaultPath: string;
    function CheckUpdate(Cancel: TCancelToken = nil): TCheckUpdateResponse;
    function CheckSelfUpdate(Cancel: TCancelToken = nil): TCheckSelfUpdateResponse;
    function SelfUpdate(Cancel: TCancelToken = nil): TSelfUpdateResponse;
    function ListRollbackVersions: TRollbackListResponse;
    function CheckDiff: TCheckDiffResponse;
    function Clean(const Options: TCleanOptions): TCleanResponse;
    function Rollback(const Version: string; const Options: TCloseOptions): TRollbackResponse;
    function ApplyUpdate(const Options: TCloseOptions): TBasicResponse;
    function DownloadUpdate(const Options: TDownloadOptions): TBasicResponse;
  end;

  TStatusCallback = procedure(const Status, Message: string) of object;
  TErrorCallback = procedure(const Message: string) of object;
  TConfirmCallback = procedure(const Version: string; Token: TCancelToken) of object;

  TUpdateClient = class;
  TUpdateThread = class;

  TUpdateClient = class
  private
    FApi: TApi;
    FCancel: TCancelToken;
    FThread: TUpdateThread;
    FLock: TMyCriticalSection;
    FStatus: string;
    FError: string;
    FOnStatus: TStatusCallback;
    FOnError: TErrorCallback;
    FOnConfirmDownload: TConfirmCallback;
    FOnConfirmApply: TConfirmCallback;
    FMustCloseProcessNames: string;
    FCloseTimeoutSeconds: Integer;
    FIdlePollSeconds: Integer;
    procedure RunLoop;
    procedure SetStatus(const Status, Message: string);
    procedure ReportError(const Message: string);
    procedure ClearError;
    function SleepFor(Milliseconds: Integer): Boolean;
    function IdleDelayMs: Integer;
    function CancelToken: TCancelToken;
  public
    constructor Create(AApi: TApi);
    destructor Destroy; override;
    procedure SetOnStatus(const Callback: TStatusCallback);
    procedure SetOnError(const Callback: TErrorCallback);
    procedure SetConfirmDownload(const Callback: TConfirmCallback);
    procedure SetConfirmApply(const Callback: TConfirmCallback);
    procedure SetMustCloseProcessNames(const Names: string);
    procedure SetCloseTimeoutSeconds(Seconds: Integer);
    procedure SetIdlePollSeconds(Seconds: Integer);
    procedure Start;
    function Stop(TimeoutMs: Integer = 5000): Boolean;
    function Status: string;
    function LastError: string;
    function IsRunning: Boolean;
  end;

  TUpdateThread = class(TThread)
  private
    FOwner: TUpdateClient;
  protected
    procedure Execute; override;
  public
    constructor Create(AOwner: TUpdateClient);
    procedure DownloadProgress(const Info: TDownloadProgressInfo);
  end;

function DefaultCloseOptions: TCloseOptions;
function DefaultCleanOptions: TCleanOptions;
function DefaultDownloadOptions: TDownloadOptions;

implementation

type
  TLineQueue = class
  private
    FLock: TMyCriticalSection;
    FLines: TStringList;
    FDone: Boolean;
  public
    constructor Create;
    destructor Destroy; override;
    procedure Push(const Line: string);
    function Pop(out Line: string): Boolean;
    procedure Finish;
    function Done: Boolean;
  end;

  TLineReaderThread = class(TThread)
  private
    FStream: TStream;
    FQueue: TLineQueue;
  protected
    procedure Execute; override;
  public
    constructor Create(AStream: TStream; AQueue: TLineQueue);
  end;

function DefaultCloseOptions: TCloseOptions;
begin
  Result.MustCloseProcessNames := '';
  Result.CloseTimeoutSeconds := 30;
end;

function DefaultCleanOptions: TCleanOptions;
begin
  Result.KeepCount := 0;
  Result.All := False;
end;

function DefaultDownloadOptions: TDownloadOptions;
begin
  Result.StallTimeoutMs := DefaultDownloadStallTimeoutMs;
  Result.OnProgress := nil;
  Result.Cancel := nil;
end;

function ReadStreamAll(Stream: TStream): string;
var
  Buffer: array[0..4095] of Byte;
  N: LongInt;
begin
  Result := '';
  if Stream = nil then
    Exit;
  while True do
  begin
    N := Stream.Read(Buffer, SizeOf(Buffer));
    if N <= 0 then
      Break;
    SetLength(Result, Length(Result) + N);
    Move(Buffer[0], Result[Length(Result) - N + 1], N);
  end;
end;

function TrimText(const Text: string): string;
begin
  Result := Trim(Text);
end;

constructor TMyCriticalSection.Create;
begin
  inherited Create;
  InitializeCriticalSection(FCS);
end;

destructor TMyCriticalSection.Destroy;
begin
  DeleteCriticalSection(FCS);
  inherited Destroy;
end;

procedure TMyCriticalSection.Enter;
begin
  EnterCriticalSection(FCS);
end;

procedure TMyCriticalSection.Leave;
begin
  LeaveCriticalSection(FCS);
end;

constructor TCancelToken.Create;
begin
  inherited Create;
  FLock := TMyCriticalSection.Create;
  FCancelled := False;
end;

destructor TCancelToken.Destroy;
begin
  FLock.Free;
  inherited Destroy;
end;

procedure TCancelToken.Cancel;
begin
  FLock.Enter;
  try
    FCancelled := True;
  finally
    FLock.Leave;
  end;
end;

function TCancelToken.IsCancelled: Boolean;
begin
  FLock.Enter;
  try
    Result := FCancelled;
  finally
    FLock.Leave;
  end;
end;

constructor TApi.Create(const Path: string);
begin
  inherited Create;
  FPath := ExpandFileName(Path);
end;

destructor TApi.Destroy;
begin
  inherited Destroy;
end;

function TApi.Path: string;
begin
  Result := FPath;
end;

class function TApi.DefaultPath: string;
begin
  Result := ExpandFileName(
    ExtractFilePath(ParamStr(0)) + '..\UpdateFolder\aly-client.exe');
end;

function TApi.StartProcess(const Args: array of string; Capture: Boolean): TProcess;
var
  I: Integer;
begin
  Result := TProcess.Create(nil);
  Result.Executable := FPath;
  Result.CurrentDirectory := ExtractFileDir(FPath);
  if Capture then
    Result.Options := [poUsePipes, poNoConsole]
  else
    Result.Options := [poNoConsole];
  for I := Low(Args) to High(Args) do
    Result.Parameters.Add(Args[I]);
  Result.Execute;
end;

function TApi.Execute(const Args: array of string; TimeoutMs: Integer; Cancel: TCancelToken;
  out StdOut, StdErr: string; out ExitCode: Integer; out TimedOut, Cancelled: Boolean;
  out StartError: string): Boolean;
var
  Proc: TProcess;
  Started: DWORD;
begin
  StdOut := '';
  StdErr := '';
  ExitCode := -1;
  TimedOut := False;
  Cancelled := False;
  StartError := '';
  Proc := nil;
  try
    try
      Proc := StartProcess(Args, True);
    except
      on E: Exception do
      begin
        StartError := E.Message;
        Exit(False);
      end;
    end;
    Started := GetTickCount;
    while Proc.Running do
    begin
      if (Cancel <> nil) and Cancel.IsCancelled then
      begin
        Cancelled := True;
        Proc.Terminate(1);
        Break;
      end;
      if (TimeoutMs > 0) and (Integer(GetTickCount - Started) >= TimeoutMs) then
      begin
        TimedOut := True;
        Proc.Terminate(1);
        Break;
      end;
      Sleep(20);
    end;
    Proc.WaitOnExit;
    ExitCode := Proc.ExitStatus;
    StdOut := ReadStreamAll(Proc.Output);
    StdErr := ReadStreamAll(Proc.Stderr);
    Result := True;
  finally
    Proc.Free;
  end;
end;

function TApi.RunCommand(const Args: array of string; TimeoutMs: Integer; Cancel: TCancelToken;
  out Root: TJSONObject; out ErrorMsg: string): Boolean;
var
  StdOut, StdErr, StartError: string;
  ExitCode: Integer;
  TimedOut, Cancelled: Boolean;
  Parsed: TJSONData;
begin
  Root := nil;
  ErrorMsg := '';
  if not Execute(Args, TimeoutMs, Cancel, StdOut, StdErr, ExitCode, TimedOut, Cancelled, StartError) then
  begin
    ErrorMsg := StartError;
    Exit(False);
  end;
  if Cancelled then
  begin
    ErrorMsg := 'aly-client call canceled';
    Exit(False);
  end;
  if TimedOut then
  begin
    ErrorMsg := Format('aly-client timed out after %ds and was killed', [TimeoutMs div 1000]);
    Exit(False);
  end;

  StdOut := TrimText(StdOut);
  if StdOut = '' then
  begin
    StdErr := TrimText(StdErr);
    if ExitCode <> 0 then
    begin
      if StdErr <> '' then
        ErrorMsg := StdErr
      else
        ErrorMsg := Format('aly-client exited with code %d (may require admin)', [ExitCode]);
    end
    else
      ErrorMsg := 'aly-client returned no output (may require admin)';
    Exit(False);
  end;

  try
    Parsed := GetJSON(StdOut);
    if not (Parsed is TJSONObject) then
    begin
      Parsed.Free;
      ErrorMsg := 'invalid JSON response';
      Exit(False);
    end;
    Root := TJSONObject(Parsed);
    if not JsonBool(Root, 'isSuccess') then
    begin
      ErrorMsg := JsonStr(Root, 'errorMsg');
      FreeAndNil(Root);
      Exit(False);
    end;
    Result := True;
  except
    on E: Exception do
    begin
      FreeAndNil(Root);
      ErrorMsg := E.Message;
      Result := False;
    end;
  end;
end;

function ExtractData(Value: TJSONObject; out DataObj: TJSONObject): Boolean;
var
  Data: TJSONData;
begin
  DataObj := nil;
  Result := False;
  Data := Value.Find('data');
  if (Data <> nil) and (Data.JSONType = jtObject) then
  begin
    DataObj := TJSONObject(Data);
    Result := True;
  end;
end;

function TApi.CheckUpdate(Cancel: TCancelToken): TCheckUpdateResponse;
var
  Root: TJSONObject;
  DataObj: TJSONObject;
begin
  Result.IsSuccess := False;
  Result.HasData := False;
  Result.ErrorMsg := '';
  if not RunCommand(['check_update'], DefaultShortTimeoutMs, Cancel, Root, Result.ErrorMsg) then
    Exit;
  try
    if ExtractData(Root, DataObj) then
    begin
      Result.Data := TCheckUpdateDataFromJson(DataObj);
      Result.HasData := True;
    end;
    Result.IsSuccess := True;
  finally
    Root.Free;
  end;
end;

function TApi.CheckSelfUpdate(Cancel: TCancelToken): TCheckSelfUpdateResponse;
var
  Root: TJSONObject;
  DataObj: TJSONObject;
begin
  Result.IsSuccess := False;
  Result.HasData := False;
  Result.ErrorMsg := '';
  if not RunCommand(['check_self_update'], DefaultShortTimeoutMs, Cancel, Root, Result.ErrorMsg) then
    Exit;
  try
    if ExtractData(Root, DataObj) then
    begin
      Result.Data := TCheckSelfUpdateDataFromJson(DataObj);
      Result.HasData := True;
    end;
    Result.IsSuccess := True;
  finally
    Root.Free;
  end;
end;

function TApi.SelfUpdate(Cancel: TCancelToken): TSelfUpdateResponse;
var
  Root: TJSONObject;
  DataObj: TJSONObject;
begin
  Result.IsSuccess := False;
  Result.HasData := False;
  Result.ErrorMsg := '';
  if not RunCommand(['self_update'], DefaultShortTimeoutMs, Cancel, Root, Result.ErrorMsg) then
    Exit;
  try
    if ExtractData(Root, DataObj) then
    begin
      Result.Data := TSelfUpdateDataFromJson(DataObj);
      Result.HasData := True;
    end;
    Result.IsSuccess := True;
  finally
    Root.Free;
  end;
end;

function TApi.ListRollbackVersions: TRollbackListResponse;
var
  Root: TJSONObject;
  DataObj: TJSONObject;
begin
  Result.IsSuccess := False;
  Result.HasData := False;
  Result.ErrorMsg := '';
  if not RunCommand(['list_rollback_versions'], DefaultShortTimeoutMs, nil, Root, Result.ErrorMsg) then
    Exit;
  try
    if ExtractData(Root, DataObj) then
    begin
      Result.Data := TRollbackListDataFromJson(DataObj);
      Result.HasData := True;
    end;
    Result.IsSuccess := True;
  finally
    Root.Free;
  end;
end;

function TApi.CheckDiff: TCheckDiffResponse;
var
  Root: TJSONObject;
  DataObj: TJSONObject;
begin
  Result.IsSuccess := False;
  Result.HasData := False;
  Result.ErrorMsg := '';
  if not RunCommand(['check_diff'], DefaultShortTimeoutMs, nil, Root, Result.ErrorMsg) then
    Exit;
  try
    if ExtractData(Root, DataObj) then
    begin
      Result.Data := TCheckDiffDataFromJson(DataObj);
      Result.HasData := True;
    end;
    Result.IsSuccess := True;
  finally
    Root.Free;
  end;
end;

function TApi.Clean(const Options: TCleanOptions): TCleanResponse;
var
  Args: array of string;
  Count: Integer;
  Root: TJSONObject;
  DataObj: TJSONObject;
begin
  Result.IsSuccess := False;
  Result.HasData := False;
  Result.ErrorMsg := '';
  Count := 1;
  SetLength(Args, Count);
  Args[0] := 'clean';
  if Options.All then
  begin
    SetLength(Args, Count + 1);
    Args[Count] := '--all';
    Inc(Count);
  end
  else if Options.KeepCount > 0 then
  begin
    SetLength(Args, Count + 2);
    Args[Count] := '--keep';
    Args[Count + 1] := IntToStr(Options.KeepCount);
    Inc(Count, 2);
  end;
  if not RunCommand(Args, DefaultShortTimeoutMs, nil, Root, Result.ErrorMsg) then
    Exit;
  try
    if ExtractData(Root, DataObj) then
    begin
      Result.Data := TCleanDataFromJson(DataObj);
      Result.HasData := True;
    end;
    Result.IsSuccess := True;
  finally
    Root.Free;
  end;
end;

function TApi.Rollback(const Version: string; const Options: TCloseOptions): TRollbackResponse;
var
  Args: array of string;
  Count: Integer;
  Root: TJSONObject;
  DataObj: TJSONObject;
begin
  Result.IsSuccess := False;
  Result.HasData := False;
  Result.ErrorMsg := '';
  Count := 3;
  SetLength(Args, Count);
  Args[0] := 'rollback';
  Args[1] := '--version';
  Args[2] := Version;
  if Options.MustCloseProcessNames <> '' then
  begin
    SetLength(Args, Count + 2);
    Args[Count] := '--must-close-process-name';
    Args[Count + 1] := Options.MustCloseProcessNames;
    Inc(Count, 2);
  end;
  if (Options.CloseTimeoutSeconds <> 0) and (Options.CloseTimeoutSeconds <> 30) then
  begin
    SetLength(Args, Count + 2);
    Args[Count] := '--close-timeout';
    Args[Count + 1] := IntToStr(Options.CloseTimeoutSeconds);
    Inc(Count, 2);
  end;
  if not RunCommand(Args, DefaultRollbackTimeoutMs, nil, Root, Result.ErrorMsg) then
    Exit;
  try
    if ExtractData(Root, DataObj) then
    begin
      Result.Data := TRollbackDataFromJson(DataObj);
      Result.HasData := True;
    end;
    Result.IsSuccess := True;
  finally
    Root.Free;
  end;
end;

function TApi.ApplyUpdate(const Options: TCloseOptions): TBasicResponse;
var
  Args: array of string;
  Count: Integer;
  Proc: TProcess;
begin
  Result.IsSuccess := False;
  Result.ErrorMsg := '';
  Count := 1;
  SetLength(Args, Count);
  Args[0] := 'apply_update';
  if Options.MustCloseProcessNames <> '' then
  begin
    SetLength(Args, Count + 2);
    Args[Count] := '--must-close-process-name';
    Args[Count + 1] := Options.MustCloseProcessNames;
    Inc(Count, 2);
  end;
  if (Options.CloseTimeoutSeconds <> 0) and (Options.CloseTimeoutSeconds <> 30) then
  begin
    SetLength(Args, Count + 2);
    Args[Count] := '--close-timeout';
    Args[Count + 1] := IntToStr(Options.CloseTimeoutSeconds);
    Inc(Count, 2);
  end;
  try
    Proc := StartProcess(Args, False);
    Proc.Free;
    Result.IsSuccess := True;
  except
    on E: Exception do
      Result.ErrorMsg := E.Message;
  end;
end;

constructor TLineQueue.Create;
begin
  inherited Create;
  FLock := TMyCriticalSection.Create;
  FLines := TStringList.Create;
  FDone := False;
end;

destructor TLineQueue.Destroy;
begin
  FLines.Free;
  FLock.Free;
  inherited Destroy;
end;

procedure TLineQueue.Push(const Line: string);
begin
  FLock.Enter;
  try
    FLines.Add(Line);
  finally
    FLock.Leave;
  end;
end;

function TLineQueue.Pop(out Line: string): Boolean;
begin
  Line := '';
  FLock.Enter;
  try
    Result := FLines.Count > 0;
    if Result then
    begin
      Line := FLines[0];
      FLines.Delete(0);
    end;
  finally
    FLock.Leave;
  end;
end;

procedure TLineQueue.Finish;
begin
  FLock.Enter;
  try
    FDone := True;
  finally
    FLock.Leave;
  end;
end;

function TLineQueue.Done: Boolean;
begin
  FLock.Enter;
  try
    Result := FDone;
  finally
    FLock.Leave;
  end;
end;

constructor TLineReaderThread.Create(AStream: TStream; AQueue: TLineQueue);
begin
  inherited Create(True);
  FreeOnTerminate := False;
  FStream := AStream;
  FQueue := AQueue;
end;

procedure TLineReaderThread.Execute;
var
  Buffer: array[0..4095] of Byte;
  N: LongInt;
  Pending: string;
  P: Integer;
  Line: string;
begin
  Pending := '';
  while not Terminated do
  begin
    N := FStream.Read(Buffer, SizeOf(Buffer));
    if N <= 0 then
      Break;
    SetLength(Pending, Length(Pending) + N);
    Move(Buffer[0], Pending[Length(Pending) - N + 1], N);
    repeat
      P := Pos(#10, Pending);
      if P = 0 then
        Break;
      Line := Copy(Pending, 1, P - 1);
      if (Line <> '') and (Line[Length(Line)] = #13) then
        SetLength(Line, Length(Line) - 1);
      FQueue.Push(Line);
      Delete(Pending, 1, P);
    until False;
  end;
  if Pending <> '' then
    FQueue.Push(Pending);
  FQueue.Finish;
end;

function TApi.DownloadUpdate(const Options: TDownloadOptions): TBasicResponse;
var
  Proc: TProcess;
  Queue: TLineQueue;
  Reader: TLineReaderThread;
  Line, ErrText: string;
  LastOutput: DWORD;
  StallMs: Integer;
  Root: TJSONObject;
  DataObj: TJSONObject;
  Progress: TDownloadProgress;
  Info: TDownloadProgressInfo;
  Parsed: TJSONData;

  procedure Cleanup;
  begin
    if Proc <> nil then
    begin
      if Proc.Running then
        Proc.Terminate(1);
      Proc.WaitOnExit;
    end;
    if Reader <> nil then
    begin
      Reader.Terminate;
      Reader.WaitFor;
      Reader.Free;
      Reader := nil;
    end;
    if Proc <> nil then
    begin
      ErrText := ReadStreamAll(Proc.Stderr);
      Proc.Free;
      Proc := nil;
    end;
    if Queue <> nil then
    begin
      Queue.Free;
      Queue := nil;
    end;
  end;

begin
  Result.IsSuccess := False;
  Result.ErrorMsg := '';
  Proc := nil;
  Queue := nil;
  Reader := nil;
  ErrText := '';
  StallMs := Options.StallTimeoutMs;
  if StallMs <= 0 then
    StallMs := DefaultDownloadStallTimeoutMs;
  try
    try
      Proc := StartProcess(['download_update'], True);
    except
      on E: Exception do
      begin
        Result.ErrorMsg := E.Message;
        Exit;
      end;
    end;
    Queue := TLineQueue.Create;
    Reader := TLineReaderThread.Create(Proc.Output, Queue);
    Reader.Start;
    LastOutput := GetTickCount;

    while True do
    begin
      if (Options.Cancel <> nil) and Options.Cancel.IsCancelled then
      begin
        Cleanup;
        Result.ErrorMsg := 'download canceled';
        Exit;
      end;

      if Queue.Pop(Line) then
      begin
        LastOutput := GetTickCount;
        if Trim(Line) = '' then
          Continue;
        try
          Parsed := GetJSON(Line);
          if not (Parsed is TJSONObject) then
          begin
            Parsed.Free;
            Cleanup;
            Result.ErrorMsg := 'invalid progress line';
            Exit;
          end;
          Root := TJSONObject(Parsed);
          try
            if not JsonBool(Root, 'isSuccess') then
            begin
              Result.ErrorMsg := JsonStr(Root, 'errorMsg');
              if Result.ErrorMsg = '' then
                Result.ErrorMsg := 'download failed';
              Cleanup;
              Exit;
            end;
            if ExtractData(Root, DataObj) then
            begin
              Progress := TDownloadProgressFromJson(DataObj);
              if (Progress.status = 'DONE') and (Progress.total > 0) and Assigned(Options.OnProgress) then
              begin
                Info.FileName := Progress.fileName;
                Info.Ratio := Progress.index / Progress.total;
                Info.Status := Progress.status;
                Info.Index := Progress.index;
                Info.Total := Progress.total;
                Info.DownloadedBytes := Progress.downloaded_bytes;
                Info.TotalBytes := Progress.total_bytes;
                Info.SpeedBps := Progress.speed_bps;
                try
                  Options.OnProgress(Info);
                except
                  // ignore callback errors
                end;
              end;
            end
            else
            begin
              Cleanup;
              Result.IsSuccess := True;
              Exit;
            end;
          finally
            Root.Free;
          end;
        except
          on E: Exception do
          begin
            Cleanup;
            Result.ErrorMsg := 'invalid progress line: ' + E.Message;
            Exit;
          end;
        end;
        Continue;
      end;

      if Queue.Done then
      begin
        Cleanup;
        if ErrText <> '' then
          Result.ErrorMsg := Trim(ErrText)
        else
          Result.ErrorMsg := 'download interrupted (aly-client exited before finish line)';
        Exit;
      end;

      if Integer(GetTickCount - LastOutput) > StallMs then
      begin
        Cleanup;
        Result.ErrorMsg := Format('download stalled: no output for %ds, process killed', [StallMs div 1000]);
        Exit;
      end;
      Sleep(20);
    end;
  except
    on E: Exception do
    begin
      try
        Cleanup;
      except
      end;
      Result.ErrorMsg := E.Message;
    end;
  end;
end;

constructor TUpdateClient.Create(AApi: TApi);
begin
  inherited Create;
  FApi := AApi;
  FCancel := nil;
  FThread := nil;
  FLock := TMyCriticalSection.Create;
  FStatus := StatusNone;
  FError := '';
  FCloseTimeoutSeconds := 30;
  FIdlePollSeconds := 300;
end;

destructor TUpdateClient.Destroy;
begin
  if not Stop(5000) then
  begin
    if FThread <> nil then
    begin
      FThread.Terminate;
      FThread.WaitFor;
      FreeAndNil(FThread);
    end;
  end;
  if (FThread = nil) and (FCancel <> nil) then
    FreeAndNil(FCancel);
  FThread.Free;
  FLock.Free;
  inherited Destroy;
end;

procedure TUpdateClient.SetOnStatus(const Callback: TStatusCallback);
begin
  FOnStatus := Callback;
end;

procedure TUpdateClient.SetOnError(const Callback: TErrorCallback);
begin
  FOnError := Callback;
end;

procedure TUpdateClient.SetConfirmDownload(const Callback: TConfirmCallback);
begin
  FOnConfirmDownload := Callback;
end;

procedure TUpdateClient.SetConfirmApply(const Callback: TConfirmCallback);
begin
  FOnConfirmApply := Callback;
end;

procedure TUpdateClient.SetMustCloseProcessNames(const Names: string);
begin
  FMustCloseProcessNames := Names;
end;

procedure TUpdateClient.SetCloseTimeoutSeconds(Seconds: Integer);
begin
  FCloseTimeoutSeconds := Seconds;
end;

procedure TUpdateClient.SetIdlePollSeconds(Seconds: Integer);
begin
  FIdlePollSeconds := Seconds;
end;

function TUpdateClient.CancelToken: TCancelToken;
begin
  Result := FCancel;
end;

procedure TUpdateClient.Start;
begin
  if IsRunning then
    Exit;
  if FThread <> nil then
  begin
    FThread.WaitFor;
    FreeAndNil(FThread);
  end;
  if FCancel <> nil then
    FreeAndNil(FCancel);
  FCancel := TCancelToken.Create;
  FThread := TUpdateThread.Create(Self);
  FThread.Start;
end;

function TUpdateClient.Stop(TimeoutMs: Integer): Boolean;
var
  Started: DWORD;
begin
  if FCancel <> nil then
    FCancel.Cancel;
  if FThread = nil then
    Exit(True);
  Started := GetTickCount;
  while not FThread.Finished do
  begin
    if Integer(GetTickCount - Started) >= TimeoutMs then
      Exit(False);
    Sleep(10);
  end;
  FThread.WaitFor;
  FreeAndNil(FThread);
  Result := True;
end;

function TUpdateClient.Status: string;
begin
  FLock.Enter;
  try
    Result := FStatus;
  finally
    FLock.Leave;
  end;
end;

function TUpdateClient.LastError: string;
begin
  FLock.Enter;
  try
    Result := FError;
  finally
    FLock.Leave;
  end;
end;

function TUpdateClient.IsRunning: Boolean;
begin
  Result := (FThread <> nil) and (not FThread.Finished);
end;

procedure TUpdateClient.SetStatus(const Status, Message: string);
var
  Callback: TStatusCallback;
begin
  FLock.Enter;
  try
    FStatus := Status;
    Callback := FOnStatus;
  finally
    FLock.Leave;
  end;
  if Assigned(Callback) then
  begin
    try
      Callback(Status, Message);
    except
      // ignore callback errors
    end;
  end;
end;

procedure TUpdateClient.ReportError(const Message: string);
var
  Callback: TErrorCallback;
begin
  FLock.Enter;
  try
    if FError = Message then
      Exit;
    FError := Message;
    Callback := FOnError;
  finally
    FLock.Leave;
  end;
  if Assigned(Callback) then
  begin
    try
      Callback(Message);
    except
      // ignore
    end;
  end;
end;

procedure TUpdateClient.ClearError;
var
  Callback: TErrorCallback;
begin
  FLock.Enter;
  try
    if FError = '' then
      Exit;
    FError := '';
    Callback := FOnError;
  finally
    FLock.Leave;
  end;
  if Assigned(Callback) then
  begin
    try
      Callback('');
    except
      // ignore
    end;
  end;
end;

function TUpdateClient.SleepFor(Milliseconds: Integer): Boolean;
var
  Started: DWORD;
begin
  Started := GetTickCount;
  while (FCancel <> nil) and (not FCancel.IsCancelled) do
  begin
    if Integer(GetTickCount - Started) >= Milliseconds then
      Break;
    Sleep(50);
  end;
  Result := (FCancel <> nil) and (not FCancel.IsCancelled);
end;

function TUpdateClient.IdleDelayMs: Integer;
var
  Seconds: Integer;
begin
  Seconds := FIdlePollSeconds;
  if Seconds <= 0 then
    Seconds := 300;
  if Seconds < 60 then
    Seconds := 60;
  Result := (Seconds + Integer(GetTickCount mod 61)) * 1000;
end;

procedure TUpdateClient.RunLoop;
var
  SelfCheck: TCheckSelfUpdateResponse;
  Updated: TSelfUpdateResponse;
  Check: TCheckUpdateResponse;
  Downloaded: TBasicResponse;
  Applied: TBasicResponse;
  Options: TDownloadOptions;
  CloseOpts: TCloseOptions;
  Version: string;
  Force: Boolean;
begin
  SelfCheck := FApi.CheckSelfUpdate(FCancel);
  if (not FCancel.IsCancelled) and SelfCheck.IsSuccess and SelfCheck.HasData then
  begin
    if SelfCheck.Data.need_update then
    begin
      Updated := FApi.SelfUpdate(FCancel);
      if not Updated.IsSuccess then
        ReportError(Updated.ErrorMsg);
    end;
  end;

  while (FCancel <> nil) and (not FCancel.IsCancelled) do
  begin
    Check := FApi.CheckUpdate(FCancel);
    if FCancel.IsCancelled then
      Exit;
    if not Check.IsSuccess then
    begin
      ReportError(Check.ErrorMsg);
      if not SleepFor(5000) then
        Exit;
      Continue;
    end;
    ClearError;

    if not Check.HasData then
    begin
      ReportError('check_update returned no data');
      if not SleepFor(5000) then
        Exit;
      Continue;
    end;

    if not Check.Data.has_update then
    begin
      if Status <> StatusNone then
        SetStatus(StatusNone, 'Update status cleared');
      if not SleepFor(IdleDelayMs) then
        Exit;
      Continue;
    end;

    Force := Check.Data.force_update;
    Version := Check.Data.new_version;
    { 三态分派（与 C# UpdateClient 一致）：has_update 且 need_download_update
      为 False 表示本地已下载待应用 / applying 崩溃恢复 / 回滚续跑，
      直接 apply，不下载、不询问（恢复场景自愈）。 }
    if Check.Data.need_download_update then
    begin
      if Version = '' then
        SetStatus(StatusDiscoveredUpdate, 'Found new version')
      else
        SetStatus(StatusDiscoveredUpdate, 'Found new version ' + Version);

      if not Force then
      begin
        if Assigned(FOnConfirmDownload) then
        begin
          try
            FOnConfirmDownload(Version, FCancel);
          except
            ReportError('confirm handler error');
          end;
        end;
        if FCancel.IsCancelled then
          Exit;
      end;

      SetStatus(StatusDownloadingUpdate, 'Downloading update...');
      Options := DefaultDownloadOptions;
      Options.Cancel := FCancel;
      Options.OnProgress := FThread.DownloadProgress;
      Downloaded := FApi.DownloadUpdate(Options);
      if FCancel.IsCancelled then
        Exit;
      if not Downloaded.IsSuccess then
      begin
        if Pos('already at latest version', Downloaded.ErrorMsg) > 0 then
        begin
          SetStatus(StatusNone, 'Update status cleared');
          if not SleepFor(IdleDelayMs) then
            Exit;
          Continue;
        end;
        ReportError(Downloaded.ErrorMsg);
        if not SleepFor(1000) then
          Exit;
        Continue;
      end;
    end;

    SetStatus(StatusDownloadedUpdate, 'Update downloaded');
    if not Force then
    begin
      if Assigned(FOnConfirmApply) then
      begin
        try
          FOnConfirmApply(Version, FCancel);
        except
          ReportError('confirm handler error');
        end;
      end;
      if FCancel.IsCancelled then
        Exit;
    end;

    SetStatus(StatusApplyUpdate, 'Applying update...');
    CloseOpts := DefaultCloseOptions;
    CloseOpts.MustCloseProcessNames := FMustCloseProcessNames;
    CloseOpts.CloseTimeoutSeconds := FCloseTimeoutSeconds;
    Applied := FApi.ApplyUpdate(CloseOpts);
    if not Applied.IsSuccess then
      ReportError('apply spawn failed: ' + Applied.ErrorMsg);
    if not SleepFor(30000) then
      Exit;
  end;
end;

constructor TUpdateThread.Create(AOwner: TUpdateClient);
begin
  inherited Create(True);
  FreeOnTerminate := False;
  FOwner := AOwner;
end;

procedure TUpdateThread.DownloadProgress(const Info: TDownloadProgressInfo);
var
  Message: string;
begin
  Message := Format('Downloading %s... %d%%', [Info.FileName, Round(Info.Ratio * 100)]);
  FOwner.SetStatus(StatusDownloadingUpdate, Message);
end;

procedure TUpdateThread.Execute;
begin
  try
    FOwner.RunLoop;
  except
    on E: Exception do
      FOwner.ReportError(E.Message);
  end;
end;

end.