using System;
using AlyClient.CSharpSDK;
using AlyClient.CSharpSDK.Models;

namespace MinimalHost
{
    internal static class Program
    {
        private static void Main()
        {
            // Requires aly-client.exe and aly-client-ui.exe in ..\UpdateFolder\.
            AlyUpdateUIClient ui = new AlyUpdateUIClient();
            AlyResponse response = ui.LaunchAuto();
            if (!response.IsSuccess)
            {
                Console.Error.WriteLine(response.ErrorMsg);
            }
        }
    }
}