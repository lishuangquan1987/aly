unit jsonhelp;

{$mode delphi}{$H+}

interface

uses
  fpjson;

type
  TStringArray = array of string;
  TBoolArray = array of Boolean;
  TInt64Array = array of Int64;
  TDoubleArray = array of Double;

function JsonStr(Value: TJSONObject; const Key: string): string;
function JsonBool(Value: TJSONObject; const Key: string): Boolean;
function JsonInt64(Value: TJSONObject; const Key: string): Int64;
function JsonFloat(Value: TJSONObject; const Key: string): Double;

function JsonStrArray(Value: TJSONObject; const Key: string): TStringArray;
function JsonBoolArray(Value: TJSONObject; const Key: string): TBoolArray;
function JsonIntArray(Value: TJSONObject; const Key: string): TInt64Array;
function JsonFloatArray(Value: TJSONObject; const Key: string): TDoubleArray;

implementation

function JsonStr(Value: TJSONObject; const Key: string): string;
var
  Data: TJSONData;
begin
  Result := '';
  if Value = nil then
    Exit;
  Data := Value.Find(Key);
  if (Data <> nil) and (Data.JSONType = jtString) then
    Result := Data.AsString;
end;

function JsonBool(Value: TJSONObject; const Key: string): Boolean;
var
  Data: TJSONData;
begin
  Result := False;
  if Value = nil then
    Exit;
  Data := Value.Find(Key);
  if (Data <> nil) and (Data.JSONType = jtBoolean) then
    Result := Data.AsBoolean;
end;

function JsonInt64(Value: TJSONObject; const Key: string): Int64;
var
  Data: TJSONData;
begin
  Result := 0;
  if Value = nil then
    Exit;
  Data := Value.Find(Key);
  if (Data <> nil) and (Data.JSONType = jtNumber) then
    Result := Data.AsInt64;
end;

function JsonFloat(Value: TJSONObject; const Key: string): Double;
var
  Data: TJSONData;
begin
  Result := 0.0;
  if Value = nil then
    Exit;
  Data := Value.Find(Key);
  if (Data <> nil) and (Data.JSONType = jtNumber) then
    Result := Data.AsFloat;
end;

function JsonStrArray(Value: TJSONObject; const Key: string): TStringArray;
var
  Data: TJSONData;
  ArrayData: TJSONArray;
  I: Integer;
  Item: TJSONData;
begin
  Result := nil;
  if Value = nil then
    Exit;
  Data := Value.Find(Key);
  if (Data = nil) or (Data.JSONType <> jtArray) then
    Exit;
  ArrayData := TJSONArray(Data);
  SetLength(Result, ArrayData.Count);
  for I := 0 to ArrayData.Count - 1 do
  begin
    Item := ArrayData.Items[I];
    if Item.JSONType = jtString then
      Result[I] := Item.AsString
    else
      Result[I] := '';
  end;
end;

function JsonBoolArray(Value: TJSONObject; const Key: string): TBoolArray;
var
  Data: TJSONData;
  ArrayData: TJSONArray;
  I: Integer;
  Item: TJSONData;
begin
  Result := nil;
  if Value = nil then
    Exit;
  Data := Value.Find(Key);
  if (Data = nil) or (Data.JSONType <> jtArray) then
    Exit;
  ArrayData := TJSONArray(Data);
  SetLength(Result, ArrayData.Count);
  for I := 0 to ArrayData.Count - 1 do
  begin
    Item := ArrayData.Items[I];
    if Item.JSONType = jtBoolean then
      Result[I] := Item.AsBoolean
    else
      Result[I] := False;
  end;
end;

function JsonIntArray(Value: TJSONObject; const Key: string): TInt64Array;
var
  Data: TJSONData;
  ArrayData: TJSONArray;
  I: Integer;
  Item: TJSONData;
begin
  Result := nil;
  if Value = nil then
    Exit;
  Data := Value.Find(Key);
  if (Data = nil) or (Data.JSONType <> jtArray) then
    Exit;
  ArrayData := TJSONArray(Data);
  SetLength(Result, ArrayData.Count);
  for I := 0 to ArrayData.Count - 1 do
  begin
    Item := ArrayData.Items[I];
    if Item.JSONType = jtNumber then
      Result[I] := Item.AsInt64
    else
      Result[I] := 0;
  end;
end;

function JsonFloatArray(Value: TJSONObject; const Key: string): TDoubleArray;
var
  Data: TJSONData;
  ArrayData: TJSONArray;
  I: Integer;
  Item: TJSONData;
begin
  Result := nil;
  if Value = nil then
    Exit;
  Data := Value.Find(Key);
  if (Data = nil) or (Data.JSONType <> jtArray) then
    Exit;
  ArrayData := TJSONArray(Data);
  SetLength(Result, ArrayData.Count);
  for I := 0 to ArrayData.Count - 1 do
  begin
    Item := ArrayData.Items[I];
    if Item.JSONType = jtNumber then
      Result[I] := Item.AsFloat
    else
      Result[I] := 0.0;
  end;
end;

end.