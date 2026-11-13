program conformance;

{$mode delphi}{$H+}

uses
  SysUtils, Classes, Windows,
  alyclient, models_gen;

type
  TCaseProc = procedure;

  TCallbacks = class
  public
    Statuses: TStringList;
    Downloads: TStringList;
    Applies: TStringList;
    constructor Create;
    destructor Destroy; override;
    procedure OnStatus(const Status, Message: string);
    procedure OnError(const Message: string);
    procedure OnConfirmDownload(const Version: string; Token: TCancelToken);
    procedure OnConfirmApply(const Version: string; Token: TCancelToken);
    procedure OnBlockingConfirm(const Version: string; Token: TCancelToken);
  end;

  TProgressSink = class
  public
    Count: Integer;
    FileName: string;
    Ratio: Double;
    Status: string;
    DownloadedBytes: Int64;
    procedure OnProgress(const Info: TDownloadProgressInfo);
  end;

var
  FakePath: string;
  ScenarioDir: string;
  PassCount: Integer;
  FailCount: Integer;

constructor TCallbacks.Create;
begin
  inherited Create;
  Statuses := TStringList.Create;
  Downloads := TStringList.Create;
  Applies := TStringList.Create;
end;

destructor TCallbacks.Destroy;
begin
  Statuses.Free;
  Downloads.Free;
  Applies.Free;
  inherited Destroy;
end;

procedure TCallbacks.OnStatus(const Status, Message: string);
begin
  Statuses.Add(Status);
end;

procedure TCallbacks.OnError(const Message: string);
begin
  // 测试只关心最终 LastError
end;

procedure TCallbacks.OnConfirmDownload(const Version: string; Token: TCancelToken);
begin
  Downloads.Add(Version);
end;

procedure TCallbacks.OnConfirmApply(const Version: string; Token: TCancelToken);
begin
  Applies.Add(Version);
end;

procedure TCallbacks.OnBlockingConfirm(const Version: string; Token: TCancelToken);
begin
  while not Token.IsCancelled do
    Sleep(10);
end;

procedure TProgressSink.OnProgress(const Info: TDownloadProgressInfo);
begin
  Inc(Count);
  FileName := Info.FileName;
  Ratio := Info.Ratio;
  Status := Info.Status;
  DownloadedBytes := Info.DownloadedBytes;
end;

procedure Check(const Name: string; Body: TCaseProc);
begin
  try
    Body;
    Inc(PassCount);
    WriteLn('PASS: ', Name);
  except
    on E: Exception do
    begin
      Inc(FailCount);
      WriteLn('FAIL: ', Name, ' - ', E.Message);
    end;
  end;
end;

procedure AssertTrue(Condition: Boolean; const Message: string);
begin
  if not Condition then
    raise Exception.Create(Message);
end;

function ScenarioPath(const Scenario: string): string;
begin
  Result := ScenarioDir + '\' + Scenario + '.json';
end;

function ApiFor(const Scenario: string): TApi;
begin
  SetEnvironmentVariable(PChar('ALY_FAKE_SCENARIO'), PChar(ScenarioPath(Scenario)));
  Result := TApi.Create(FakePath);
end;

procedure WaitStatus(Client: TUpdateClient; const Want: string; TimeoutMs: Integer);
var
  Started: DWORD;
begin
  Started := GetTickCount;
  while Integer(GetTickCount - Started) < TimeoutMs do
  begin
    if Client.Status = Want then
      Exit;
    Sleep(20);
  end;
  raise Exception.CreateFmt('waitStatus timeout, last=%s, error=%s',
    [Client.Status, Client.LastError]);
end;

procedure TestCheckUpdateAvailable;
var
  Api: TApi;
  Response: TCheckUpdateResponse;
begin
  Api := ApiFor('update_available');
  try
    Response := Api.CheckUpdate;
    AssertTrue(Response.IsSuccess, Response.ErrorMsg);
    AssertTrue(Response.HasData, 'no data');
    AssertTrue(Response.Data.has_update, 'has_update false');
    AssertTrue(Response.Data.need_download_update, 'need_download_update false');
    AssertTrue(Response.Data.new_version = '1.0.1', 'new_version');
    AssertTrue(not Response.Data.force_update, 'force_update');
    AssertTrue(Length(Response.Data.release_notes) = 2, 'release_notes');
  finally
    Api.Free;
  end;
end;

procedure TestCheckLatestOffline;
var
  Api: TApi;
  Response: TCheckUpdateResponse;
begin
  Api := ApiFor('already_latest');
  try
    Response := Api.CheckUpdate;
    AssertTrue(Response.IsSuccess, Response.ErrorMsg);
    AssertTrue(Response.HasData and (not Response.Data.has_update), 'already_latest');
  finally
    Api.Free;
  end;
  Api := ApiFor('offline');
  try
    Response := Api.CheckUpdate;
    AssertTrue(Response.IsSuccess, Response.ErrorMsg);
    AssertTrue(Response.HasData and (not Response.Data.has_update), 'offline');
  finally
    Api.Free;
  end;
end;

procedure TestDownloadProgress;
var
  Api: TApi;
  Options: TDownloadOptions;
  Sink: TProgressSink;
  Response: TBasicResponse;
begin
  Api := ApiFor('update_available');
  Sink := TProgressSink.Create;
  try
    Options := DefaultDownloadOptions;
    Options.OnProgress := Sink.OnProgress;
    Response := Api.DownloadUpdate(Options);
    AssertTrue(Response.IsSuccess, Response.ErrorMsg);
    AssertTrue(Sink.Count = 1, 'progress count');
    AssertTrue(Sink.FileName = 'app.exe', 'file');
    AssertTrue(Sink.Ratio = 1.0, 'ratio');
    AssertTrue(Sink.Status = 'DONE', 'status');
    AssertTrue(Sink.DownloadedBytes = 1000, 'downloadedBytes');
  finally
    Sink.Free;
    Api.Free;
  end;
end;

procedure TestDownloadPartialFailure;
var
  Api: TApi;
  Options: TDownloadOptions;
  Response: TBasicResponse;
begin
  Api := ApiFor('download_partial_failure');
  try
    Options := DefaultDownloadOptions;
    Response := Api.DownloadUpdate(Options);
    AssertTrue(not Response.IsSuccess, 'expected failure');
    AssertTrue(Pos('checksum mismatch', Response.ErrorMsg) > 0, Response.ErrorMsg);
  finally
    Api.Free;
  end;
end;

procedure TestDownloadStall;
var
  Api: TApi;
  Options: TDownloadOptions;
  Response: TBasicResponse;
begin
  Api := ApiFor('download_stall');
  try
    Options := DefaultDownloadOptions;
    Options.StallTimeoutMs := 1000;
    Response := Api.DownloadUpdate(Options);
    AssertTrue(not Response.IsSuccess, 'expected failure');
    AssertTrue(Pos('stalled', Response.ErrorMsg) > 0, Response.ErrorMsg);
  finally
    Api.Free;
  end;
end;

procedure TestApplyImmediate;
var
  Api: TApi;
  Options: TCloseOptions;
  Response: TBasicResponse;
  Started: DWORD;
begin
  Api := ApiFor('update_available');
  try
    Options := DefaultCloseOptions;
    Options.MustCloseProcessNames := 'A,B';
    Started := GetTickCount;
    Response := Api.ApplyUpdate(Options);
    AssertTrue(Response.IsSuccess, Response.ErrorMsg);
    AssertTrue(Integer(GetTickCount - Started) < 10000, 'apply took too long');
  finally
    Api.Free;
  end;
end;

procedure TestSelfUpdate;
var
  Api: TApi;
  CheckResponse: TCheckSelfUpdateResponse;
  UpdateResponse: TSelfUpdateResponse;
begin
  Api := ApiFor('self_update');
  try
    CheckResponse := Api.CheckSelfUpdate;
    AssertTrue(CheckResponse.IsSuccess, CheckResponse.ErrorMsg);
    AssertTrue(CheckResponse.HasData and CheckResponse.Data.need_update, 'need_update');
    UpdateResponse := Api.SelfUpdate;
    AssertTrue(UpdateResponse.IsSuccess, UpdateResponse.ErrorMsg);
    AssertTrue(UpdateResponse.HasData and UpdateResponse.Data.updated, 'updated');
    AssertTrue(UpdateResponse.Data.old_path = 'aly-client.exe.old', 'old_path');
  finally
    Api.Free;
  end;
end;

procedure TestRollback;
var
  Api: TApi;
  ListResponse: TRollbackListResponse;
  RollbackResponse: TRollbackResponse;
  Options: TCloseOptions;
begin
  Api := ApiFor('rollback');
  try
    ListResponse := Api.ListRollbackVersions;
    AssertTrue(ListResponse.IsSuccess, ListResponse.ErrorMsg);
    AssertTrue(ListResponse.HasData and (Length(ListResponse.Data.versions) = 2), 'versions');
    Options := DefaultCloseOptions;
    RollbackResponse := Api.Rollback('1.0.0', Options);
    AssertTrue(RollbackResponse.IsSuccess, RollbackResponse.ErrorMsg);
    AssertTrue(RollbackResponse.HasData and (RollbackResponse.Data.version = '1.0.0'), 'rollback');
  finally
    Api.Free;
  end;
end;

procedure TestDiffAndClean;
var
  Api: TApi;
  DiffResponse: TCheckDiffResponse;
  CleanResponse: TCleanResponse;
  CleanOptions: TCleanOptions;
begin
  Api := ApiFor('check_diff');
  try
    DiffResponse := Api.CheckDiff;
    AssertTrue(DiffResponse.IsSuccess, DiffResponse.ErrorMsg);
    AssertTrue(DiffResponse.HasData and (Length(DiffResponse.Data.files) = 1), 'files');
    AssertTrue(DiffResponse.Data.files[0].path = 'app.exe', 'path');
  finally
    Api.Free;
  end;

  Api := ApiFor('clean_all');
  try
    CleanOptions := DefaultCleanOptions;
    CleanOptions.All := True;
    CleanResponse := Api.Clean(CleanOptions);
    AssertTrue(CleanResponse.IsSuccess, CleanResponse.ErrorMsg);
    AssertTrue(CleanResponse.HasData and (CleanResponse.Data.mode = 'all'), 'clean mode');
  finally
    Api.Free;
  end;
end;

procedure TestMissingExecutable;
var
  Api: TApi;
  Response: TCheckUpdateResponse;
begin
  Api := TApi.Create('C:\aly-pascal-missing\aly-client.exe');
  try
    Response := Api.CheckUpdate;
    AssertTrue(not Response.IsSuccess, 'expected failure');
    AssertTrue(Response.ErrorMsg <> '', 'empty error');
  finally
    Api.Free;
  end;
end;

procedure TestDefaultPath;
begin
  AssertTrue(Pos('UpdateFolder', TApi.DefaultPath) > 0, TApi.DefaultPath);
end;

procedure TestManagedForce;
var
  Api: TApi;
  Client: TUpdateClient;
  Callbacks: TCallbacks;
  I: Integer;
  SeenDownloading, SeenDownloaded: Boolean;
begin
  Api := ApiFor('force_update');
  Client := TUpdateClient.Create(Api);
  Callbacks := TCallbacks.Create;
  try
    Client.SetOnStatus(Callbacks.OnStatus);
    Client.SetOnError(Callbacks.OnError);
    Client.Start;
    WaitStatus(Client, StatusApplyUpdate, 10000);
    AssertTrue(Client.Stop(10000), 'stop timeout');
    AssertTrue(Callbacks.Statuses.Count > 0, 'no statuses');
    AssertTrue(Callbacks.Statuses[0] = StatusDiscoveredUpdate, 'first status');
    SeenDownloading := False;
    SeenDownloaded := False;
    for I := 0 to Callbacks.Statuses.Count - 1 do
    begin
      if Callbacks.Statuses[I] = StatusDownloadingUpdate then
        SeenDownloading := True;
      if Callbacks.Statuses[I] = StatusDownloadedUpdate then
        SeenDownloaded := True;
    end;
    AssertTrue(SeenDownloading, 'missing downloading');
    AssertTrue(SeenDownloaded, 'missing downloaded');
    AssertTrue(Callbacks.Statuses[Callbacks.Statuses.Count - 1] = StatusApplyUpdate, 'last status');
    AssertTrue(Client.LastError = '', Client.LastError);
  finally
    Client.Free;
    Callbacks.Free;
    Api.Free;
  end;
end;

procedure TestManagedNonForce;
var
  Api: TApi;
  Client: TUpdateClient;
  Callbacks: TCallbacks;
begin
  Api := ApiFor('update_available');
  Client := TUpdateClient.Create(Api);
  Callbacks := TCallbacks.Create;
  try
    Client.SetConfirmDownload(Callbacks.OnConfirmDownload);
    Client.SetConfirmApply(Callbacks.OnConfirmApply);
    Client.Start;
    WaitStatus(Client, StatusApplyUpdate, 10000);
    AssertTrue(Client.Stop(10000), 'stop timeout');
    AssertTrue((Callbacks.Downloads.Count = 1) and (Callbacks.Downloads[0] = '1.0.1'), 'downloads');
    AssertTrue((Callbacks.Applies.Count = 1) and (Callbacks.Applies[0] = '1.0.1'), 'applies');
  finally
    Client.Free;
    Callbacks.Free;
    Api.Free;
  end;
end;

procedure TestManagedPendingApply;
var
  Api: TApi;
  Client: TUpdateClient;
  Callbacks: TCallbacks;
begin
  Api := ApiFor('pending_apply');
  Client := TUpdateClient.Create(Api);
  Callbacks := TCallbacks.Create;
  try
    Client.SetOnStatus(Callbacks.OnStatus);
    Client.SetOnError(Callbacks.OnError);
    Client.SetConfirmDownload(Callbacks.OnConfirmDownload);
    Client.SetConfirmApply(Callbacks.OnConfirmApply);
    Client.Start;
    WaitStatus(Client, StatusApplyUpdate, 10000);
    AssertTrue(Client.Stop(10000), 'stop timeout');
    AssertTrue(Callbacks.Downloads.Count = 0, 'pending apply must not ask for download');
    AssertTrue(Callbacks.Statuses.IndexOf(StatusDiscoveredUpdate) < 0, 'discovered status');
    AssertTrue(Callbacks.Statuses.IndexOf(StatusDownloadingUpdate) < 0, 'downloading status');
    AssertTrue((Callbacks.Applies.Count = 1) and (Callbacks.Applies[0] = '1.0.1'), 'applies');
    AssertTrue((Callbacks.Statuses.Count > 0) and (Callbacks.Statuses[0] = StatusDownloadedUpdate), 'first status');
    AssertTrue(Callbacks.Statuses[Callbacks.Statuses.Count - 1] = StatusApplyUpdate, 'last status');
    AssertTrue(Client.LastError = '', Client.LastError);
  finally
    Client.Free;
    Callbacks.Free;
    Api.Free;
  end;
end;

procedure TestManagedOffline;
var
  Api: TApi;
  Client: TUpdateClient;
  Callbacks: TCallbacks;
begin
  Api := ApiFor('offline');
  Client := TUpdateClient.Create(Api);
  Callbacks := TCallbacks.Create;
  try
    Client.SetConfirmDownload(Callbacks.OnConfirmDownload);
    Client.Start;
    Sleep(500);
    AssertTrue(Client.Status = StatusNone, Client.Status);
    AssertTrue(Client.Stop(10000), 'stop timeout');
    AssertTrue(Callbacks.Downloads.Count = 0, 'confirm should not be called');
  finally
    Client.Free;
    Callbacks.Free;
    Api.Free;
  end;
end;

procedure TestStopPendingConfirm;
var
  Api: TApi;
  Client: TUpdateClient;
  Callbacks: TCallbacks;
  Started: DWORD;
begin
  Api := ApiFor('update_available');
  Client := TUpdateClient.Create(Api);
  Callbacks := TCallbacks.Create;
  try
    Client.SetConfirmDownload(Callbacks.OnBlockingConfirm);
    Client.Start;
    WaitStatus(Client, StatusDiscoveredUpdate, 10000);
    Started := GetTickCount;
    AssertTrue(Client.Stop(10000), 'stop timeout');
    AssertTrue(Integer(GetTickCount - Started) < 3000, 'stop too slow');
  finally
    Client.Free;
    Callbacks.Free;
    Api.Free;
  end;
end;

procedure TestStopIdempotent;
var
  Api: TApi;
  Client: TUpdateClient;
begin
  Api := ApiFor('offline');
  Client := TUpdateClient.Create(Api);
  try
    Client.Start;
    Sleep(100);
    AssertTrue(Client.Stop(10000), 'first stop');
    AssertTrue(Client.Stop(10000), 'second stop');
    AssertTrue(not Client.IsRunning, 'isRunning');
  finally
    Client.Free;
    Api.Free;
  end;
end;

procedure TestCancelToken;
var
  Token: TCancelToken;
begin
  Token := TCancelToken.Create;
  try
    AssertTrue(not Token.IsCancelled, 'new token');
    Token.Cancel;
    AssertTrue(Token.IsCancelled, 'cancel ignored');
  finally
    Token.Free;
  end;
end;

begin
  if ParamCount < 2 then
  begin
    WriteLn('usage: conformance.exe <fake-aly-client.exe> <scenariosDir>');
    Halt(2);
  end;
  FakePath := ParamStr(1);
  ScenarioDir := ParamStr(2);
  SetConsoleOutputCP(CP_UTF8);
  PassCount := 0;
  FailCount := 0;

  Check('check_update available', TestCheckUpdateAvailable);
  Check('check_update latest/offline', TestCheckLatestOffline);
  Check('download_update progress', TestDownloadProgress);
  Check('download_update partial failure', TestDownloadPartialFailure);
  Check('download_update stall watchdog', TestDownloadStall);
  Check('apply_update returns immediately', TestApplyImmediate);
  Check('self_update flow', TestSelfUpdate);
  Check('rollback flow', TestRollback);
  Check('check_diff and clean', TestDiffAndClean);
  Check('missing executable returns failure', TestMissingExecutable);
  Check('defaultPath points to UpdateFolder', TestDefaultPath);
  Check('managed force update full chain', TestManagedForce);
  Check('managed non-force waits for confirmations', TestManagedNonForce);
  Check('managed pending apply applies without downloading', TestManagedPendingApply);
  Check('managed offline stays idle', TestManagedOffline);
  Check('stop interrupts pending confirmation', TestStopPendingConfirm);
  Check('stop is idempotent', TestStopIdempotent);
  Check('cancel token behavior', TestCancelToken);

  WriteLn('SUMMARY: PASS=', PassCount, ' FAIL=', FailCount);
  if FailCount = 0 then
    Halt(0)
  else
    Halt(1);
end.