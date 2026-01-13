package utils

// func getClient() (*minio.Client, error) {
// 	endpoint := os.Getenv("SP_URL")
// 	accessKey := os.Getenv("SP_ACCESS_KEY_ID")
// 	secretKey := os.Getenv("SP_SECRET_ACCESS_KEY")

// 	client, err := minio.New(endpoint, &minio.Options{
// 		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
// 		Secure: true,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	return client, nil
// }

// func SaveImageToSpaces(objectName, imageData string) error {
// 	// Decode the base64 data
// 	decodedData, err := base64.StdEncoding.DecodeString(imageData)
// 	if err != nil {
// 		return err
// 	}
// 	bucketName := os.Getenv("SP_BUCKET")

// 	// Get the Minio client
// 	client, err := getClient()
// 	if err != nil {
// 		return err
// 	}

// 	// Upload the decoded image data to the specified object name in your Space
// 	contentType := "image/jpeg"

// 	_, err = client.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(decodedData), int64(len(decodedData)), minio.PutObjectOptions{
// 		ContentType: contentType,
// 		UserMetadata: map[string]string{
// 			"x-amz-acl": "public-read",
// 		},
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// func UploadFileToSpace(objectName string, fileContent io.Reader) error {
// 	// Get file content
// 	fileData, err := ioutil.ReadAll(fileContent)
// 	if err != nil {
// 		return fmt.Errorf("failed to read file content: %v", err)
// 	}

// 	mimeType := http.DetectContentType(fileData)

// 	// Manually set MIME type for .docx and .xlsx files
// 	if mimeType == "application/zip" {
// 		if strings.HasSuffix(objectName, ".docx") {
// 			mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
// 		} else if strings.HasSuffix(objectName, ".xlsx") {
// 			mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
// 		}
// 	}

// 	// Define the allowed MIME types for each file type
// 	allowedMimeTypes := map[string]bool{
// 		"application/pdf":          true,
// 		"application/msword":       true,
// 		"application/vnd.ms-excel": true,
// 		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
// 		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
// 		"image/jpeg": true,
// 		"image/png":  true,
// 		// Add more MIME types as needed
// 	}
// fmt.Println("mimeType",mimeType)
// 	fmt.Println("allwo",allowedMimeTypes)
// 	// Check if the MIME type is allowed
// 	if !allowedMimeTypes[mimeType] {
// 		fmt.Println("allwo",allowedMimeTypes)
// 		return fmt.Errorf("unsupported file type: %s", mimeType)
// 	}
// 	// Get the Minio client
// 	client, err := getClient()
// 	if err != nil {
// 		return err
// 	}

// 	// Create a context with a timeout
// 	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// 	defer cancel()

// 	// Upload the file to DigitalOcean Space
// 	_, err = client.PutObject(ctx, os.Getenv("SP_BUCKET"), objectName, bytes.NewReader(fileData), int64(len(fileData)), minio.PutObjectOptions{
// 		ContentType: mimeType,
// 		UserMetadata: map[string]string{
// 			"x-amz-acl": "public-read",
// 		},
// 	})
// 	if err != nil {
// 		return fmt.Errorf("failed to upload file to DigitalOcean Space: %v", err)
// 	}

// 	return nil
// }

// func DeleteImageFromSpaces(objectName string) error {
// 	// Get the Minio client
// 	client, err := getClient()
// 	if err != nil {
// 		return err
// 	}

// 	bucketName := os.Getenv("SP_BUCKET")

// 	// Remove the specified object from your Space
// 	err = client.RemoveObject(context.Background(), bucketName, objectName, minio.RemoveObjectOptions{})
// 	if err != nil {
// 		// Check if the error is due to the object not existing
// 		if strings.Contains(err.Error(), "The specified key does not exist") {
// 			fmt.Print("Object does not exist:", objectName)
// 			return nil
// 		}
// 		return err
// 	}

// 	fmt.Print("Object deleted successfully:", objectName)
// 	return nil
// }

// func ObjectExists(objectName string) (bool, error) {
// 	// Get the Minio client
// 	client, err := getClient()
// 	if err != nil {
// 		return false, err
// 	}

// 	bucketName := os.Getenv("SP_BUCKET")

// 	// StatObject is used to check the existence of an object without downloading its content
// 	_, err = client.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
// 	if err != nil {
// 		if strings.Contains(err.Error(), "The specified key does not exist") {
// 			return false, nil // Object does not exist
// 		}
// 		return false, err // Other error
// 	}

// 	return true, nil // Object exists
// }

// // check if image exists on the internet
// func CheckImageExistInCloud(imageURL string) error {

// 	resp, err := http.Head(imageURL)
// 	if err != nil {
// 		return errors.New("invalid image url")
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode == http.StatusOK {
// 		return nil
// 	}

// 	return errors.New("image does not exist")
// }
