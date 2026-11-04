/// mirabase — the owned MiraBase Dart client.
///
/// Entry: forApplication(baseUrl) → immutable withContext({databaseId}) /
/// withTenant({namespace,key}) views. Inherited collection methods remain
/// on the underlying inherited client; the additive typed services are
/// delivered alongside their owning gates.
library mirabase;

export 'src/context.dart'
    show
        MirabaseClient,
        MirabaseContextView,
        MirabaseTenantView,
        MirabaseException,
        forApplication;
