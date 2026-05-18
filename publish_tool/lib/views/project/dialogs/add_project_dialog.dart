import 'dart:io';
import 'package:file_picker/file_picker.dart';
import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:publish_tool/dto/project_dto.dart';
import 'package:publish_tool/models/project_config.dart';
import 'package:publish_tool/api/project_api.dart';
import 'package:publish_tool/viewmodels/app_controller.dart';

class AddProjectDialog extends StatefulWidget {
  const AddProjectDialog({super.key});

  @override
  State<AddProjectDialog> createState() => _AddProjectDialogState();
}

class _AddProjectDialogState extends State<AddProjectDialog> {
  final _serverUrlCtrl = TextEditingController(text: 'http://');
  final _localPathCtrl = TextEditingController();
  final _exePathCtrl = TextEditingController();

  List<ProjectDto> _remoteProjects = [];
  ProjectDto? _selectedProject;
  bool _fetchingProjects = false;
  String? _fetchError;

  bool _loading = false;
  String? _error;

  @override
  void dispose() {
    _serverUrlCtrl.dispose();
    _localPathCtrl.dispose();
    _exePathCtrl.dispose();
    super.dispose();
  }

  Future<void> _fetchProjects() async {
    final url = _serverUrlCtrl.text.trim();
    if (url.isEmpty || url == 'http://') return;
    setState(() {
      _fetchingProjects = true;
      _fetchError = null;
      _remoteProjects = [];
      _selectedProject = null;
    });

    List<ProjectDto>? result;
    String? error;
    try {
      final resp = await ProjectApi(url).getAllProjects();
      if (!resp.isSuccess) throw resp.errorMsg ?? 'Unknown error';
      result = resp.data ?? [];
    } catch (e) {
      error = e.toString();
    }

    if (mounted) {
      setState(() {
        _fetchingProjects = false;
        _fetchError = error;
        if (result != null) _remoteProjects = result;
      });
    }
  }

  Future<void> _pickFolder() async {
    final result = await FilePicker.platform.getDirectoryPath();
    if (result == null) return;
    _localPathCtrl.text = result;
    // 自动寻找文件夹内第一个 exe
    final exeFiles = Directory(result)
        .listSync()
        .whereType<File>()
        .where((f) => f.path.toLowerCase().endsWith('.exe'))
        .toList();
    _exePathCtrl.text = exeFiles.isNotEmpty ? exeFiles.first.path : '';
    setState(() {});
  }

  Future<void> _pickExe() async {
    final folder = _localPathCtrl.text.trim();
    final result = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: ['exe'],
      initialDirectory: folder.isNotEmpty ? folder : null,
    );
    if (result == null || result.files.single.path == null) return;
    final path = result.files.single.path!;
    if (folder.isNotEmpty && !path.startsWith(folder)) {
      setState(() => _error = 'exe 文件必须在所选文件夹内');
      return;
    }
    _exePathCtrl.text = path;
    setState(() => _error = null);
  }

  Future<void> _submit() async {
    if (_selectedProject == null) {
      setState(() => _error = '请先获取并选择服务端项目');
      return;
    }
    if (_localPathCtrl.text.trim().isEmpty) {
      setState(() => _error = '请选择本地文件夹');
      return;
    }
    setState(() { _loading = true; _error = null; });
    try {
      final config = ProjectConfig(
        serverId: _selectedProject!.id,
        name: _selectedProject!.name,
        title: _selectedProject!.title,
        serverUrl: _serverUrlCtrl.text.trim(),
        exePath: _exePathCtrl.text.trim(),
        localPath: _localPathCtrl.text.trim(),
      );
      await Get.find<AppController>().addLocalProject(config);
      if (mounted) Navigator.pop(context);
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return ContentDialog(
      title: const Text('新建客户端项目'),
      constraints: const BoxConstraints(maxWidth: 500),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Text(_error!, style: const TextStyle(color: Color(0xFFcc3333))),
            ),
          // 服务器地址 + 获取按钮
          _label('服务器地址 *'),
          Row(
            children: [
              Expanded(
                child: TextBox(
                  controller: _serverUrlCtrl,
                  placeholder: 'http://10.96.115.14:2002',
                ),
              ),
              const SizedBox(width: 4),
              Button(
                onPressed: _fetchingProjects ? null : _fetchProjects,
                child: _fetchingProjects
                    ? const SizedBox(width: 14, height: 14, child: ProgressRing(strokeWidth: 2))
                    : const Text('获取项目'),
              ),
            ],
          ),
          if (_fetchError != null)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(_fetchError!, style: const TextStyle(color: Color(0xFFcc3333), fontSize: 11)),
            ),
          const SizedBox(height: 8),
          // 项目下拉（始终显示，获取后刷新数据源）
          _label('选择项目 *'),
          _ProjectDropdown(
            projects: _remoteProjects,
            selected: _selectedProject,
            onChanged: (v) => setState(() => _selectedProject = v),
          ),
          const SizedBox(height: 8),
          // 本地文件夹
          _label('本地文件夹 *'),
          Row(
            children: [
              Expanded(child: TextBox(controller: _localPathCtrl, readOnly: true)),
              const SizedBox(width: 4),
              IconButton(
                icon: const Icon(FluentIcons.folder_open),
                onPressed: _pickFolder,
              ),
            ],
          ),
          const SizedBox(height: 8),
          // exe 路径
          _label('exe 路径'),
          Row(
            children: [
              Expanded(child: TextBox(controller: _exePathCtrl)),
              const SizedBox(width: 4),
              IconButton(
                icon: const Icon(FluentIcons.document_search),
                onPressed: _localPathCtrl.text.isNotEmpty ? _pickExe : null,
              ),
            ],
          ),
          const Text('exe 必须在所选文件夹内',
              style: TextStyle(fontSize: 11, color: Color(0xFF888888))),
        ],
      ),
      actions: [
        FilledButton(
          onPressed: _loading ? null : _submit,
          child: _loading
              ? const SizedBox(width: 16, height: 16, child: ProgressRing(strokeWidth: 2))
              : const Text('确定'),
        ),
        Button(onPressed: () => Navigator.pop(context), child: const Text('取消')),
      ],
    );
  }

  Widget _label(String text) => Padding(
        padding: const EdgeInsets.only(bottom: 4),
        child: Text(text, style: const TextStyle(fontSize: 12)),
      );
}

class _ProjectDropdown extends StatefulWidget {
  final List<ProjectDto> projects;
  final ProjectDto? selected;
  final ValueChanged<ProjectDto> onChanged;

  const _ProjectDropdown({
    required this.projects,
    required this.selected,
    required this.onChanged,
  });

  @override
  State<_ProjectDropdown> createState() => _ProjectDropdownState();
}

class _ProjectDropdownState extends State<_ProjectDropdown> {
  final _flyoutController = FlyoutController();

  @override
  void dispose() {
    _flyoutController.dispose();
    super.dispose();
  }

  void _showMenu() {
    _flyoutController.showFlyout(
      builder: (ctx) => MenuFlyout(
        items: widget.projects
            .map((p) => MenuFlyoutItem(
                  text: Text('${p.title}（${p.name}）'),
                  onPressed: () {
                    Flyout.of(ctx).close();
                    widget.onChanged(p);
                  },
                ))
            .toList(),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final label = widget.selected != null
        ? '${widget.selected!.title}（${widget.selected!.name}）'
        : (widget.projects.isEmpty ? '请先点击"获取项目"' : '请选择项目');

    return SizedBox(
      width: double.infinity,
      child: FlyoutTarget(
        controller: _flyoutController,
        child: Button(
          onPressed: widget.projects.isEmpty ? null : _showMenu,
          child: Row(
            children: [
              Expanded(child: Text(label, overflow: TextOverflow.ellipsis)),
              const Icon(FluentIcons.chevron_down, size: 12),
            ],
          ),
        ),
      ),
    );
  }
}
