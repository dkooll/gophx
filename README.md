# gophx

This repository is dedicated to experimenting with various small Go projects and refactoring existing ones.

It explores different techniques and implementations, focusing on improving code quality and efficiency.

Some projects may grow into ongoing initiatives, but the primary focus is on experimentation.

## Notes

For now, the approach is to define small interfaces with the methods you want to implement.

Next, create structs that store the data and implement these interfaces, ensuring a clear separation of concerns.

Encapsulate core logic within the methods to keep the implementation clean and modular.

Group related behaviors into distinct interfaces to maintain single responsibility.

When broader functionality is needed, combine small, focused interfaces by embedding them into a larger one.

Keep related data and its operations together in the same package to promote cohesion and discoverability.
