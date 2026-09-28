using Common.Utilities;

var context = new Config.BuildContext(Context);

Task("Default")
    .Description("Shows this output")
    .Does(() => TaskHelp.Show(context, Tasks));

Task("SetMatrix")
    .Does(() => new Config.Tasks.SetMatrix().Run(context));

return BuildRunner.Run(context, () => RunTarget(Argument("target", "Default")));
