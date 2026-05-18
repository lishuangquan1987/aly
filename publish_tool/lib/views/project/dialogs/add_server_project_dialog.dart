import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:publish_tool/dto/create_project_dto.dart';
import 'package:publish_tool/viewmodels/app_controller.dart';

class AddServerProjectDialog extends StatefulWidget {
  const AddServerProjectDialog({super.key});

  @override
  State<AddServerProjectDialog> createState() => _AddServerProjectDialogState();
}

class _AddServerProjectDialogState extends State<AddServerProjectDialog> {
  final _serverUrlCtrl = TextEditingController(text: 'http://');
  final _nameCtrl = TextEditingController();
  final _titleCtrl = TextEditingController();
  bool _loading = false;
  String? _error;

  @override
  void dispose() {
    _serverUrlCtrl.dispose();
    _nameCtrl.dispose();
    _titleCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final url = _serverUrlCtrl.text.trim();
    final name = _nameCtrl.text.trim();
    final title = _titleCtrl.text.trim();
    if (url.isEmpty || name.isEmpty || title.isEmpty) {
      setState(() => _error = '服务器地址、项目名称、显示标题均为必填项');
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final dto = CreateProjectDto(
        name: name,
        title: title,
        isForceUpdate: false,
        ignoreFolders: [],
        ignoreFiles: [],
      );
      final res = await Get.find<AppController>().createServerProject(url, dto);
      if (!res.isSuccess) {
        setState(() => _error = res.errorMsg ?? '创建失败');
        return;
      }
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
      title: const Text('新建服务器项目'),
      constraints: const BoxConstraints(maxWidth: 440),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Text(_error!,
                  style: const TextStyle(color: Color(0xFFcc3333))),
            ),
          _field('服务器地址 *', _serverUrlCtrl, 'http://10.96.115.14:2002'),
          _field('项目名称 *', _nameCtrl, '唯一标识，如 YOFC.iMES-Q.Client'),
          _field('显示标题 *', _titleCtrl, '如 石英MES客户端'),
        ],
      ),
      actions: [
        FilledButton(
          onPressed: _loading ? null : _submit,
          child: _loading
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: ProgressRing(strokeWidth: 2))
              : const Text('确定'),
        ),
        Button(
            onPressed: () => Navigator.pop(context),
            child: const Text('取消')),
      ],
    );
  }

  Widget _field(
      String label, TextEditingController ctrl, String placeholder) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: const TextStyle(fontSize: 12)),
          const SizedBox(height: 4),
          TextBox(controller: ctrl, placeholder: placeholder),
        ],
      ),
    );
  }
}
